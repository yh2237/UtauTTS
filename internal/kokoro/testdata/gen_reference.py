import argparse
import json
import time
from pathlib import Path

import kokoro
import torch


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("pth")
    parser.add_argument("voice")
    args = parser.parse_args()
    torch.set_num_threads(8)
    phonemes = "hˈɛloʊ wˈɜɹld, ðɪs ɪz ə tˈɛst."
    model = kokoro.KModel(repo_id="hexgrad/Kokoro-82M", config=str(Path(args.pth).parent / "config.json"), model=args.pth)
    model.eval()
    voice = torch.load(args.voice, map_location="cpu", weights_only=True)
    captured = {}
    original = model.predictor.F0Ntrain

    def f0n(x, s):
        f0, noise = original(x, s)
        captured["f0"] = f0.detach().numpy().astype("<f4")
        return f0, noise

    model.predictor.F0Ntrain = f0n
    torch.manual_seed(42)
    start = time.perf_counter()
    with torch.no_grad():
        out = model(phonemes, voice[len(phonemes) - 1], speed=1, return_output=True)
    elapsed = time.perf_counter() - start
    root = Path(__file__).resolve().parent
    root.mkdir(parents=True, exist_ok=True)
    captured["f0"].tofile(root / "f0.f32")
    out.audio.detach().numpy().astype("<f4").tofile(root / "audio.f32")
    (root / "durations.json").write_text(json.dumps(out.pred_dur.tolist()) + "\n", encoding="utf-8")
    (root / "tokens.json").write_text(json.dumps([model.vocab[p] for p in phonemes if p in model.vocab]) + "\n", encoding="utf-8")
    print("durations", out.pred_dur.tolist(), "F0", captured["f0"].size, "audio", out.audio.numel(), "time", elapsed)


if __name__ == "__main__":
    main()
