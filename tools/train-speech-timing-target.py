"""時間伸縮の目標モデル（speech-timing-target）を学習し、Goの同梱用safetensorsへ書き出す。

目標モデルは音素・音素の中の位置・対数の長さ・相対の対数F0・有声から、発話ごとに正規化した
80帯域の対数メル包絡（40Hz〜12kHz、10msフレーム）を予測する。合成時は包絡の形を使わず、
WORLD出力の包絡とのDTWで時間の割り当てだけを決める（internal/speechtiming）。

学習データはMFAで整列した日本語の読み上げ（version-1 JSONL の id・audio_path と、
<alignments>/<id>.json の phones 層）。同梱モデル v1 は抑揚モデル v10 と同じ
つくよみちゃんコーパス Vol.1 とみんなで作るJSUT BASIC5000_0001-0600 で学習した。

例:
  python tools/train-speech-timing-target.py --dataset out/mfa-align-20261002/base-mfa.jsonl \\
    --alignments out/mfa-align-20261002/alignments --world-engine runtime/utautts-world-engine.dll
"""
import argparse
import ctypes
import json
import math
import random
import struct
import wave
from pathlib import Path

import numpy as np
import torch
from torch import nn

FRAME_MS = 10.0
MELS = 80
EMBED = 32
HIDDEN = 128
KERNEL = 5
DILATIONS = (1, 2, 4, 8, 1, 2, 4, 8)
DROPOUT = 0.15
PHONES = ["<pad>", "<unk>", "sil", "a", "i", "u", "e", "o", "N", "cl",
          "k", "g", "s", "sh", "z", "j", "t", "ch", "ts", "d", "n", "h", "f", "b", "p", "m", "y", "r", "w", "v",
          "ky", "gy", "ny", "hy", "my", "ry", "by", "py", "dy", "ty"]
PHONE_ID = {p: i for i, p in enumerate(PHONES)}

# MFA（japanese_mfa、IPA）からOpen JTalk形式へ。無声化した母音は合成時に区別できないので有声の母音へまとめる。
IPA = {"a": "a", "i": "i", "ɯ": "u", "e": "e", "o": "o", "aː": "a", "iː": "i", "ɯː": "u", "eː": "e", "oː": "o",
       "i̥": "i", "ɯ̥": "u", "ɴ": "N", "ʔ": "cl", "k": "k", "ɡ": "g", "s": "s", "ɕ": "sh", "z": "z", "dʑ": "j",
       "t": "t", "tɕ": "ch", "ts": "ts", "d": "d", "n": "n", "h": "h", "ɸ": "f", "b": "b", "p": "p", "m": "m",
       "j": "y", "ɾ": "r", "w": "w", "v": "v", "dʲ": "dy", "tʲ": "ty", "ʑ": "j", "dz": "z", "ŋ": "n", "ɰ̃": "N"}
# 口蓋化した子音は、次が「い」ならそのまま、それ以外は拗音。
PALATAL = {"c": ("k", "ky"), "ɟ": ("g", "gy"), "ɲ": ("n", "ny"), "ç": ("h", "hy"), "mʲ": ("m", "my"),
           "ɾʲ": ("r", "ry"), "bʲ": ("b", "by"), "pʲ": ("p", "py")}


class World:
    """UtauTTSのWORLDエンジン（runtime/utautts-world-engine）で分析する。"""

    class Shape(ctypes.Structure):
        _fields_ = [("sample_count", ctypes.c_int), ("sample_rate", ctypes.c_int), ("frame_period_ms", ctypes.c_double),
                    ("frame_count", ctypes.c_int), ("fft_size", ctypes.c_int)]

    D = ctypes.POINTER(ctypes.c_double)

    class Analysis(ctypes.Structure):
        pass

    def __init__(self, path):
        self.dll = ctypes.CDLL(str(Path(path).resolve()))
        self.Analysis._fields_ = [("samples", self.D), ("sample_count", ctypes.c_int), ("sample_rate", ctypes.c_int),
                                  ("frame_period_ms", ctypes.c_double), ("input_f0", self.D), ("input_f0_count", ctypes.c_int),
                                  ("f0", self.D), ("spectrum", self.D), ("aperiodicity", self.D)]
        self.dll.UtauTTSWorldAnalysisShape.argtypes = [ctypes.POINTER(self.Shape), ctypes.c_char_p, ctypes.c_int]
        self.dll.UtauTTSWorldAnalyze.argtypes = [ctypes.POINTER(self.Analysis), ctypes.c_char_p, ctypes.c_int]

    def analyze(self, x, rate):
        x = np.ascontiguousarray(x, dtype=np.float64)
        error = ctypes.create_string_buffer(512)
        shape = self.Shape(len(x), rate, FRAME_MS, 0, 0)
        if not self.dll.UtauTTSWorldAnalysisShape(ctypes.byref(shape), error, 512):
            raise RuntimeError(error.value)
        frames, bins = shape.frame_count, shape.fft_size // 2 + 1
        f0, sp, ap = np.zeros(frames), np.zeros(frames * bins), np.zeros(frames * bins)
        ptr = lambda a: a.ctypes.data_as(self.D)  # noqa: E731
        request = self.Analysis(ptr(x), len(x), rate, FRAME_MS, None, 0, ptr(f0), ptr(sp), ptr(ap))
        if not self.dll.UtauTTSWorldAnalyze(ctypes.byref(request), error, 512):
            raise RuntimeError(error.value)
        return f0, sp.reshape(frames, bins), shape.fft_size


def read_wav(path):
    with wave.open(str(path)) as handle:
        rate, channels, width = handle.getframerate(), handle.getnchannels(), handle.getsampwidth()
        if width != 2:
            raise ValueError(f"{path}: 16-bit PCM only")
        x = np.frombuffer(handle.readframes(handle.getnframes()), dtype=np.int16).astype(np.float64) / 32768
    return rate, x.reshape(-1, channels).mean(axis=1)


def log_mel(sp, rate, fft_size):
    """internal/speechtimingのlogMelと同じ三角フィルタ。"""
    freqs = np.arange(fft_size // 2 + 1) * rate / fft_size
    mel = lambda hz: 2595 * np.log10(1 + hz / 700)  # noqa: E731
    points = 700 * (10 ** (np.linspace(mel(40), mel(12000), MELS + 2) / 2595) - 1)
    matrix = np.zeros((MELS, len(freqs)))
    for i in range(MELS):
        lo, mid, hi = points[i], points[i + 1], points[i + 2]
        matrix[i] = np.clip(np.minimum((freqs - lo) / (mid - lo), (hi - freqs) / (hi - mid)), 0, None)
    matrix /= np.maximum(matrix.sum(axis=1, keepdims=True), 1e-12)
    return 10 * np.log10(sp @ matrix.T + 1e-12)


def mfa_phones(path):
    entries = json.loads(Path(path).read_text(encoding="utf-8"))["tiers"]["phones"]["entries"]
    filled, cursor = [], 0.0
    for start, end, label in entries:
        if float(start) > cursor + 1e-4:
            filled.append((cursor, float(start), ""))
        filled.append((float(start), float(end), label))
        cursor = float(end)
    phones = []
    for index, (start, end, label) in enumerate(filled):
        if not label or label in ("<eps>", "sil", "sp", "spn"):
            name = "sil"
        elif label in PALATAL:
            following = filled[index + 1][2] if index + 1 < len(filled) else ""
            name = PALATAL[label][0] if following in ("i", "iː", "i̥") else PALATAL[label][1]
        else:
            name = IPA.get(label, "<unk>")
        phones.append((start, end, name))
    return phones


def frame_inputs(phones, f0):
    """internal/speechtimingのframeInputsと同じ入力。"""
    frames = len(f0)
    ids = np.full((frames, 3), PHONE_ID["sil"], dtype=np.int64)
    cont = np.zeros((frames, 4), dtype=np.float32)
    for index, (start, end, label) in enumerate(phones):
        a, b = int(round(start * 1000 / FRAME_MS)), int(round(end * 1000 / FRAME_MS))
        a, b = max(0, a), min(frames, max(b, a + 1))
        if a >= frames:
            break
        previous = phones[index - 1][2] if index > 0 else "sil"
        following = phones[index + 1][2] if index + 1 < len(phones) else "sil"
        ids[a:b] = [PHONE_ID[label], PHONE_ID[previous], PHONE_ID[following]]
        cont[a:b, 0] = (np.arange(b - a) + 0.5) / max(1, b - a)
        cont[a:b, 1] = math.log((end - start) * 1000 + 1) / 6
    voiced = f0 > 0
    if voiced.any():
        logs = np.log(f0[voiced])
        cont[:, 2] = np.interp(np.arange(frames), np.flatnonzero(voiced), logs - logs.mean()) / 0.3
    cont[:, 3] = voiced
    return ids, cont


def featurize(world, record, alignments):
    rate, x = read_wav(record["audio_path"])
    f0, sp, fft = world.analyze(x, rate)
    ids, cont = frame_inputs(mfa_phones(Path(alignments) / f"{record['id']}.json"), f0)
    mel = log_mel(sp, rate, fft)
    speech = ids[:, 0] != PHONE_ID["sil"]
    if speech.sum() < 2:
        speech[:] = True
    target = (mel - mel[speech].mean(axis=0)) / (mel[speech].std(axis=0) + 1e-3)
    return {"id": record["id"], "ids": ids, "cont": cont, "target": target.astype(np.float32)}


class Target(nn.Module):
    def __init__(self):
        super().__init__()
        self.phone = nn.Embedding(len(PHONES), EMBED)
        self.inp = nn.Conv1d(3 * EMBED + 4, HIDDEN, 1)
        self.blocks = nn.ModuleList(nn.Conv1d(HIDDEN, HIDDEN, KERNEL, dilation=d, padding=d * (KERNEL // 2)) for d in DILATIONS)
        self.norms = nn.ModuleList(nn.LayerNorm(HIDDEN) for _ in DILATIONS)
        self.out = nn.Conv1d(HIDDEN, MELS, 1)

    def forward(self, ids, cont):
        h = self.inp(torch.cat([self.phone(ids).flatten(2), cont], dim=2).transpose(1, 2))
        for block, norm in zip(self.blocks, self.norms):
            y = norm(block(h).transpose(1, 2)).transpose(1, 2)
            h = h + nn.functional.dropout(nn.functional.gelu(y), DROPOUT, self.training)
        return self.out(h).transpose(1, 2)


def train(data, args):
    torch.manual_seed(args.seed)
    rng = random.Random(args.seed)
    order = list(range(len(data)))
    rng.shuffle(order)
    valid = [data[i] for i in order[:args.valid]]
    training = [data[i] for i in order[args.valid:]]
    device = "cuda" if torch.cuda.is_available() else "cpu"
    model = Target().to(device)
    optimizer = torch.optim.AdamW(model.parameters(), lr=2e-3, weight_decay=1e-4)
    scheduler = torch.optim.lr_scheduler.OneCycleLR(optimizer, max_lr=2e-3, total_steps=args.steps, pct_start=0.1)
    window, batch = 400, 16

    def sample():
        ids_b, cont_b, target_b = [], [], []
        for _ in range(batch):
            item = training[rng.randrange(len(training))]
            start = rng.randrange(max(1, len(item["ids"]) - window))
            part = slice(start, start + window)
            pad = window - len(item["ids"][part])
            ids_b.append(np.pad(item["ids"][part], ((0, pad), (0, 0))))
            cont_b.append(np.pad(item["cont"][part], ((0, pad), (0, 0))))
            target_b.append(np.pad(item["target"][part], ((0, pad), (0, 0)), constant_values=np.nan))
        return [torch.tensor(np.stack(values), device=device) for values in (ids_b, cont_b, target_b)]

    def evaluate():
        model.eval()
        errors = []
        with torch.no_grad():
            for item in valid:
                pred = model(torch.tensor(item["ids"][None], device=device), torch.tensor(item["cont"][None], device=device))[0].cpu().numpy()
                speech = item["ids"][:, 0] != PHONE_ID["sil"]
                errors.append(np.abs(pred[speech] - item["target"][speech]).mean())
        model.train()
        return float(np.mean(errors))

    best, best_state, best_step = float("inf"), None, 0
    for step in range(args.steps):
        ids, cont, target = sample()
        mask = ~torch.isnan(target[..., 0])
        loss = (model(ids, cont) - torch.nan_to_num(target)).abs()[mask].mean()
        optimizer.zero_grad()
        loss.backward()
        nn.utils.clip_grad_norm_(model.parameters(), 1.0)
        optimizer.step()
        scheduler.step()
        if step % 250 == 0 or step == args.steps - 1:
            score = evaluate()
            if score < best:
                best, best_step = score, step
                best_state = {k: v.detach().cpu().clone() for k, v in model.state_dict().items()}
            if step % 1000 == 0:
                print(f"step {step} loss {loss.item():.3f} valid {score:.3f} best {best:.3f}@{best_step}", flush=True)
    model.load_state_dict(best_state)
    return model.cpu().eval(), best, best_step


def write_safetensors(path, tensors, metadata):
    header, offset, blobs = {"__metadata__": metadata}, 0, []
    for name in sorted(tensors):
        blob = np.ascontiguousarray(tensors[name], dtype="<f4").tobytes()
        header[name] = {"dtype": "F32", "shape": list(tensors[name].shape), "data_offsets": [offset, offset + len(blob)]}
        offset += len(blob)
        blobs.append(blob)
    encoded = json.dumps(header, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    encoded += b" " * ((8 - len(encoded) % 8) % 8)
    with open(path, "wb") as handle:
        handle.write(struct.pack("<Q", len(encoded)) + encoded + b"".join(blobs))


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--dataset", required=True, help="version-1 JSONL with id and audio_path")
    parser.add_argument("--alignments", required=True, help="directory of MFA <id>.json alignments")
    parser.add_argument("--world-engine", default="runtime/utautts-world-engine.dll")
    parser.add_argument("--cache", default="out/speech-timing-target/features.pt")
    parser.add_argument("--out", default="internal/speechtiming/speech-timing-target-v1.safetensors")
    parser.add_argument("--fixture", default="internal/speechtiming/testdata/target-v1-parity.json")
    parser.add_argument("--training-corpus", default="Tsukuyomi-chan Corpus Vol.1 (VOICEACTRESS100) + Minnade JSUT Corpus basic5000 BASIC5000_0001-0600, aligned with Montreal Forced Aligner japanese_mfa")
    parser.add_argument("--license-notice", action="append", default=[])
    parser.add_argument("--steps", type=int, default=6000)
    parser.add_argument("--valid", type=int, default=30)
    parser.add_argument("--seed", type=int, default=0)
    args = parser.parse_args()
    cache = Path(args.cache)
    if cache.exists():
        data = torch.load(cache, weights_only=False)
    else:
        world = World(args.world_engine)
        records = [json.loads(line) for line in Path(args.dataset).read_text(encoding="utf-8").splitlines() if line.strip()]
        records = [r for r in records if (Path(args.alignments) / f"{r['id']}.json").exists()]
        data = [featurize(world, record, args.alignments) for record in records]
        cache.parent.mkdir(parents=True, exist_ok=True)
        torch.save(data, cache)
    print("utterances", len(data), "frames", sum(len(d["ids"]) for d in data), flush=True)
    model, best, best_step = train(data, args)
    notices = args.license_notice or ["licenses/TSUKUYOMI-CORPUS.txt", "licenses/MINNADE-JSUT-CORPUS.txt", "licenses/MFA-Japanese-NOTICE.txt"]
    metadata = {
        "id": "speech-timing-target-v1", "format": "utautts-speech-timing-tcn-1", "phones": " ".join(PHONES),
        "kernel": str(KERNEL), "dilations": " ".join(map(str, DILATIONS)), "frame_ms": str(FRAME_MS), "mels": str(MELS),
        "license": "MIT License", "training_corpus": args.training_corpus, "license_notices": " ".join(notices),
        "valid_l1": f"{best:.4f}", "steps": str(best_step),
    }
    write_safetensors(args.out, {k: v.numpy() for k, v in model.state_dict().items()}, metadata)
    rng = np.random.default_rng(3)
    ids = rng.integers(0, len(PHONES), size=(37, 3))
    cont = rng.normal(size=(37, 4)).astype(np.float32)
    with torch.no_grad():
        output = model(torch.tensor(ids[None]), torch.tensor(cont[None]))[0].numpy()
    Path(args.fixture).parent.mkdir(parents=True, exist_ok=True)
    Path(args.fixture).write_text(json.dumps({"ids": ids.tolist(), "cont": cont.tolist(), "output": output.tolist()}), encoding="utf-8")
    print(f"wrote {args.out} (valid L1 {best:.4f} at step {best_step})")


if __name__ == "__main__":
    main()
