"""Web版の参照照合のモデルを書き出す。KokoroとWavLMをONNXにし、重みはfloat16で持つ（読み込み時にCastでfloat32へ戻す）。
usage: python tools/web-reference/export.py <models/reference> <models/wavlm> <out dir>
要るもの: torch、onnx、kokoro（Apache-2.0）、transformers。

<out>/<Kokoroの重みの名前>/duration.onnx  input_ids[1,T], ref_s[1,256], speed[1] -> duration[T]（丸める前）
<out>/<Kokoroの重みの名前>/prosody.onnx   input_ids[1,T], ref_s[1,256], pred_dur[T] -> f0[F]
<out>/<Kokoroの重みの名前>/synth.onnx     input_ids[1,T], ref_s[1,256], pred_dur[T] -> audio[N]（24 kHz）, f0[F]
<out>/wavlm6.onnx                         audio[1,N]（16 kHz） -> features[T,768]（WavLM Base+ の6層目）
<out>/voices/<声>.f32                     声の表（510×256、float32）
<out>/fs/                                 Goの仮想FSに置くファイル（設定、辞書、inference.json）"""
import json, os, shutil, sys
import numpy as np, onnx, torch
from onnx import helper, numpy_helper, TensorProto
from kokoro.model import KModel
from transformers import WavLMModel

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


def fp16_weights(path):
    """float32の重みをfloat16で持ち、Castでfloat32へ戻す。演算はfloat32のまま。"""
    m = onnx.load(path); g = m.graph
    keep, casts = [], []
    for init in g.initializer:
        if init.data_type == TensorProto.FLOAT and np.prod(init.dims) >= 1024:
            h = numpy_helper.from_array(numpy_helper.to_array(init).astype(np.float16), init.name + "__f16")
            keep.append(h); casts.append(helper.make_node("Cast", [h.name], [init.name], to=TensorProto.FLOAT))
        else:
            keep.append(init)
    del g.initializer[:]; g.initializer.extend(keep)
    nodes = list(g.node); del g.node[:]; g.node.extend(casts + nodes)
    onnx.save(m, path)


def export(module, args, path, inputs, outputs, axes):
    with torch.no_grad():
        torch.onnx.export(module, args, path, input_names=inputs, output_names=outputs, dynamic_axes=axes, opset_version=17, dynamo=False)
    fp16_weights(path)
    print("wrote", path, os.path.getsize(path) >> 20, "MB", flush=True)


def main():
    reference, wavlm, out = sys.argv[1:4]
    os.makedirs(os.path.join(out, "voices"), exist_ok=True)
    fs = os.path.join(out, "fs")
    models, voices = [], []
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
    m = WavLMModel.from_pretrained(wavlm).eval()
    m.encoder.layers = m.encoder.layers[:6]
    export(Layer6(m), (torch.randn(1, 32000),), os.path.join(out, "wavlm6.onnx"), ["audio"], ["features"], {"audio": {1: "N"}, "features": {0: "T"}})
    json.dump({"models": models, "voices": voices}, open(os.path.join(fs, "inference.json"), "w"))
    print("models", models, "voices", voices)


if __name__ == "__main__":
    main()
