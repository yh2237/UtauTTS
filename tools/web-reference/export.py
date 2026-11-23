"""Web版の参照照合のモデルを書き出す。Kokoroと音声エンコーダをONNXにし、重みはfloat16で持つ（読み込み時にCastでfloat32へ戻す）。
usage: python tools/web-reference/export.py <models/reference> <models/wavlm> <out dir> [--espeak ...] [--whisper ...]
要るもの: torch、onnx、kokoro（Apache-2.0）、transformers。

<out>/<Kokoroの重みの名前>/duration.onnx      input_ids[1,T], ref_s[1,256], speed[1] -> duration[T]（丸める前）
<out>/<Kokoroの重みの名前>/prosody.onnx       input_ids[1,T], ref_s[1,256], pred_dur[T] -> f0[F]
<out>/<Kokoroの重みの名前>/synth.onnx         input_ids[1,T], ref_s[1,256], pred_dur[T] -> audio[N]（24 kHz）, f0[F]
<out>/wavlm6.onnx                             audio[1,N]（16 kHz） -> features[T,768]（WavLM Base+ の6層目）
<out>/wav2vec2-espeak-l20.onnx                audio[1,N]（16 kHz） -> features[T,1024]（任意・Espeak 20層目）
<out>/whisper-small-l12-posinv.onnx           audio[1,32000]（2秒窓） -> features[50,768]（任意・Whisper 12層目 PosInv）
<out>/voices/<声>.f32                         声の表（510×256、float32）
<out>/fs/                                     Goの仮想FSに置くファイル（設定、辞書、inference.json）"""
import argparse, json, math, os, shutil, sys
import numpy as np, onnx, torch
from onnx import helper, numpy_helper, TensorProto
from transformers import Wav2Vec2Config, Wav2Vec2Model, WavLMModel, WhisperConfig, WhisperModel

KOKOROS = [("kokoro-v1_0.pth", "config.json", "hexgrad/Kokoro-82M", ["af_heart", "ef_dora", "pf_dora"], ""),
           ("kokoro-zh/kokoro-v1_1-zh.pth", "kokoro-zh/config.json", "hexgrad/Kokoro-82M-v1.1-zh", ["zf_001"], "kokoro-zh/")]


def encode(k, input_ids, ref_s):
    T = input_ids.shape[1]
    mask = torch.zeros((1, T), dtype=torch.bool)
    lengths = torch.full((1,), T, dtype=torch.long)
    bert = k.bert(input_ids, attention_mask=(~mask).int())
    d = k.predictor.text_encoder(k.bert_encoder(bert).transpose(-1, -2), ref_s[:, 128:], lengths, mask)
    return d, mask, lengths


def align(T, pred_dur):
    indices = torch.repeat_interleave(torch.arange(T), pred_dur)
    aln = torch.zeros((T, indices.shape[0]))
    aln[indices, torch.arange(indices.shape[0])] = 1
    return aln.unsqueeze(0)


class Duration(torch.nn.Module):
    def __init__(self, k):
        super().__init__(); self.k = k

    def forward(self, input_ids, ref_s, speed):
        d, _, _ = encode(self.k, input_ids, ref_s)
        x, _ = self.k.predictor.lstm(d)
        return (torch.sigmoid(self.k.predictor.duration_proj(x)).sum(axis=-1) / speed).squeeze(0)


class Prosody(torch.nn.Module):
    def __init__(self, k):
        super().__init__(); self.k = k

    def forward(self, input_ids, ref_s, pred_dur):
        d, _, _ = encode(self.k, input_ids, ref_s)
        f0, _ = self.k.predictor.F0Ntrain(d.transpose(-1, -2) @ align(input_ids.shape[1], pred_dur), ref_s[:, 128:])
        return f0.squeeze(0)


class Synth(torch.nn.Module):
    def __init__(self, k):
        super().__init__(); self.k = k

    def forward(self, input_ids, ref_s, pred_dur):
        k = self.k
        d, mask, lengths = encode(k, input_ids, ref_s)
        aln = align(input_ids.shape[1], pred_dur)
        f0, n = k.predictor.F0Ntrain(d.transpose(-1, -2) @ aln, ref_s[:, 128:])
        audio = k.decoder(k.text_encoder(input_ids, lengths, mask) @ aln, f0, n, ref_s[:, :128]).squeeze()
        return audio, f0.squeeze(0)


class Layer6(torch.nn.Module):
    def __init__(self, m):
        super().__init__(); self.m = m

    def forward(self, audio):
        return self.m(audio, output_hidden_states=True).hidden_states[6].squeeze(0)


class EspeakLayer20(torch.nn.Module):
    def __init__(self, m, do_normalize):
        super().__init__(); self.m = m; self.do_normalize = do_normalize

    def forward(self, audio):
        if self.do_normalize:
            mean = audio.mean(dim=-1, keepdim=True)
            var = ((audio - mean) ** 2).mean(dim=-1, keepdim=True)
            audio = (audio - mean) / torch.sqrt(var + 1e-7)
        return self.m(audio, output_hidden_states=True).hidden_states[20].squeeze(0)


def slaney_mel_filters(mels=80, sample_rate=16000, n_fft=400):
    bins = n_fft // 2 + 1
    linear_step = 200.0 / 3.0
    log_step = math.log(6.4) / 27.0
    mel_max = 15.0 + math.log(8.0) / log_step
    edges = []
    for i in range(mels + 2):
        mel = mel_max * i / (mels + 1)
        hz = linear_step * mel if mel < 15.0 else 1000.0 * math.exp(log_step * (mel - 15.0))
        edges.append(hz)
    filters = np.zeros((mels, bins), dtype=np.float32)
    for b in range(bins):
        hz = b * sample_rate / float(n_fft)
        for m in range(mels):
            left, center, right = edges[m], edges[m + 1], edges[m + 2]
            rise = (hz - left) / (center - left)
            fall = (right - hz) / (right - center)
            filters[m, b] = max(0.0, min(rise, fall)) * 2.0 / (right - left)
    return torch.from_numpy(filters)


def whisper_stft_conv_weight(n_fft=400):
    bins = n_fft // 2 + 1
    window = np.hanning(n_fft + 1)[:-1].astype(np.float64)
    n = np.arange(n_fft, dtype=np.float64)[None, :]
    k = np.arange(bins, dtype=np.float64)[:, None]
    angle = 2.0 * np.pi * k * n / float(n_fft)
    real = np.cos(angle) * window[None, :]
    imag = -np.sin(angle) * window[None, :]
    weight = np.concatenate([real, imag], axis=0).astype(np.float32)[:, None, :]
    return torch.from_numpy(weight)


class WhisperPosInvWindow(torch.nn.Module):
    def __init__(self, encoder, mel_filters):
        super().__init__()
        self.encoder = encoder
        self.register_buffer("preproc_stft_weight", whisper_stft_conv_weight())
        self.register_buffer("preproc_mel_filters", mel_filters)

    def forward(self, audio):
        x = audio.unsqueeze(1)
        left_reflect = torch.flip(x[:, :, 1:201], dims=[2])
        right_zero = torch.zeros((x.shape[0], 1, 360), dtype=x.dtype, device=x.device)
        padded = torch.cat([left_reflect, x, right_zero], dim=2)
        spec = torch.nn.functional.conv1d(padded, self.preproc_stft_weight, stride=160)
        re, im = spec[:, :201, :], spec[:, 201:, :]
        power = re * re + im * im
        mel = torch.matmul(self.preproc_mel_filters, power)
        log_mel = torch.log10(torch.clamp(mel, min=1e-10))
        threshold = log_mel.max() - 8.0
        sig = (torch.clamp(log_mel, min=threshold) + 4.0) / 4.0
        sil = (torch.clamp(torch.full((1, 80, 1), -10.0, dtype=x.dtype, device=x.device), min=threshold) + 4.0) / 4.0
        mel3000 = torch.cat([sig, sil.expand(-1, -1, 3000 - sig.shape[2])], dim=2)
        hidden = self.encoder(mel3000, output_hidden_states=True).hidden_states[12]
        return hidden[0, 25:75, :]


def fp16_weights(path):
    """float32の重みをfloat16で持ち、Castでfloat32へ戻す。演算はfloat32のまま。preproc_*はfloat32を保つ。"""
    m = onnx.load(path); g = m.graph
    keep, casts = [], []
    for init in g.initializer:
        if init.data_type == TensorProto.FLOAT and np.prod(init.dims) >= 1024 and not init.name.startswith("preproc_"):
            h = numpy_helper.from_array(numpy_helper.to_array(init).astype(np.float16), init.name + "__f16")
            keep.append(h); casts.append(helper.make_node("Cast", [h.name], [init.name], to=TensorProto.FLOAT))
        else:
            keep.append(init)
    del g.initializer[:]; g.initializer.extend(keep)
    nodes = list(g.node); del g.node[:]; g.node.extend(casts + nodes)
    onnx.save(m, path)


def load_safetensors_state(path):
    from safetensors.torch import load_file
    return load_file(path)


def load_espeak_model(path, config_path, preprocessor_path):
    pre_path = preprocessor_path or (os.path.join(path, "preprocessor_config.json") if os.path.isdir(path) else os.path.join(os.path.dirname(path), "preprocessor_config.json"))
    do_normalize = True
    if os.path.isfile(pre_path):
        with open(pre_path, "r", encoding="utf-8") as f:
            do_normalize = bool(json.load(f).get("do_normalize", True))
    if os.path.isdir(path) and not config_path:
        m = Wav2Vec2Model.from_pretrained(path).eval()
    else:
        cfg_path = config_path or os.path.join(os.path.dirname(path), "config.json")
        m = Wav2Vec2Model(Wav2Vec2Config.from_json_file(cfg_path)).eval()
        weights = os.path.join(path, "model.safetensors") if os.path.isdir(path) else path
        state = load_safetensors_state(weights)
        if any(k.startswith("wav2vec2.") for k in state):
            state = {k[len("wav2vec2."):]: v for k, v in state.items() if k.startswith("wav2vec2.")}
        missing, unexpected = m.load_state_dict(state, strict=False)
        missing = [k for k in missing if "masked_spec_embed" not in k]
        unexpected = [k for k in unexpected if not k.startswith("lm_head.")]
        if missing or unexpected:
            raise RuntimeError(f"espeak state mismatch: missing={missing}, unexpected={unexpected}")
    m.encoder.layers = m.encoder.layers[:20]
    return EspeakLayer20(m, do_normalize).eval()


def load_whisper_encoder(path, config_path):
    if os.path.isdir(path) and not config_path:
        wm = WhisperModel.from_pretrained(path).eval()
        enc = wm.encoder
    else:
        cfg_path = config_path or os.path.join(os.path.dirname(path), "config.json")
        enc = WhisperModel(WhisperConfig.from_json_file(cfg_path)).encoder.eval()
        weights = os.path.join(path, "model.safetensors") if os.path.isdir(path) else path
        state = load_safetensors_state(weights)
        enc_state = {}
        for prefix in ("model.encoder.", "encoder."):
            matched = {k[len(prefix):]: v for k, v in state.items() if k.startswith(prefix)}
            if matched:
                enc_state = matched
                break
        if not enc_state:
            enc_state = state
        enc.load_state_dict(enc_state, strict=True)
    enc.layers = enc.layers[:12]
    return WhisperPosInvWindow(enc, slaney_mel_filters()).eval()


def export(module, args, path, inputs, outputs, axes):
    with torch.no_grad():
        torch.onnx.export(module, args, path, input_names=inputs, output_names=outputs, dynamic_axes=axes, opset_version=17, dynamo=False)
    fp16_weights(path)
    print("wrote", path, os.path.getsize(path) >> 20, "MB", flush=True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("reference")
    parser.add_argument("wavlm")
    parser.add_argument("out")
    parser.add_argument("--espeak", default="")
    parser.add_argument("--espeak-config", default="")
    parser.add_argument("--espeak-preprocessor", default="")
    parser.add_argument("--whisper", default="")
    parser.add_argument("--whisper-config", default="")
    parser.add_argument("--skip-kokoro", action="store_true")
    parser.add_argument("--skip-wavlm", action="store_true")
    args = parser.parse_args()

    reference, wavlm, out = args.reference, args.wavlm, args.out
    os.makedirs(os.path.join(out, "voices"), exist_ok=True)
    fs = os.path.join(out, "fs")
    os.makedirs(fs, exist_ok=True)
    inf_path = os.path.join(fs, "inference.json")
    models, voices, encoders = [], [], []
    if args.skip_kokoro and os.path.isfile(inf_path):
        with open(inf_path, "r", encoding="utf-8") as f:
            prev = json.load(f)
            models = list(prev.get("models", []))
            voices = list(prev.get("voices", []))
    if not args.skip_kokoro:
        from kokoro.model import KModel
        for weights, config, repo, voice_names, prefix in KOKOROS:
            if not os.path.exists(os.path.join(reference, weights)):
                continue
            name = os.path.splitext(os.path.basename(weights))[0]
            k = KModel(repo_id=repo, config=os.path.join(reference, config), model=os.path.join(reference, weights), disable_complex=True).eval()
            d = os.path.join(out, name); os.makedirs(d, exist_ok=True)
            ids = torch.LongTensor([[0, 50, 83, 54, 156, 57, 135, 16, 0]]); ref = torch.randn(1, 256)
            dur = torch.full((ids.shape[1],), 3, dtype=torch.long)
            axes = {"input_ids": {1: "T"}, "pred_dur": {0: "T"}}
            export(Duration(k), (ids, ref, torch.ones(1)), os.path.join(d, "duration.onnx"), ["input_ids", "ref_s", "speed"], ["duration"],
                   {"input_ids": {1: "T"}, "duration": {0: "T"}})
            export(Prosody(k), (ids, ref, dur), os.path.join(d, "prosody.onnx"), ["input_ids", "ref_s", "pred_dur"], ["f0"], {**axes, "f0": {0: "F"}})
            export(Synth(k), (ids, ref, dur), os.path.join(d, "synth.onnx"), ["input_ids", "ref_s", "pred_dur"], ["audio", "f0"],
                   {**axes, "audio": {0: "N"}, "f0": {0: "F"}})
            os.makedirs(os.path.join(fs, os.path.dirname(config)), exist_ok=True)
            shutil.copy(os.path.join(reference, config), os.path.join(fs, config))
            models.append(name)
            for voice in voice_names:
                pack = torch.load(os.path.join(reference, prefix + voice + ".pt"), weights_only=True)
                pack.reshape(-1, 256).numpy().astype("<f4").tofile(os.path.join(out, "voices", voice + ".f32"))
                voices.append(voice)
        for lexicon in ["us_gold.json", "us_silver.json"]:
            shutil.copy(os.path.join(reference, lexicon), os.path.join(fs, lexicon))
    if not args.skip_wavlm:
        m = WavLMModel.from_pretrained(wavlm).eval()
        m.encoder.layers = m.encoder.layers[:6]
        export(Layer6(m), (torch.randn(1, 32000),), os.path.join(out, "wavlm6.onnx"), ["audio"], ["features"], {"audio": {1: "N"}, "features": {0: "T"}})
    if os.path.isfile(os.path.join(out, "wavlm6.onnx")):
        encoders.append("wavlm-base-plus-l6")
    if args.espeak:
        es = load_espeak_model(args.espeak, args.espeak_config, args.espeak_preprocessor)
        export(es, (torch.randn(1, 32000),), os.path.join(out, "wav2vec2-espeak-l20.onnx"), ["audio"], ["features"], {"audio": {1: "N"}, "features": {0: "T"}})
        encoders.append("wav2vec2-espeak-l20")
    if args.whisper:
        wh = load_whisper_encoder(args.whisper, args.whisper_config)
        export(wh, (torch.randn(1, 32000),), os.path.join(out, "whisper-small-l12-posinv.onnx"), ["audio"], ["features"], None)
        encoders.append("whisper-small-l12-posinv")
    with open(inf_path, "w", encoding="utf-8") as f:
        json.dump({"models": models, "voices": voices, "encoders": encoders}, f)
    print("models", models, "voices", voices, "encoders", encoders)


if __name__ == "__main__":
    main()
