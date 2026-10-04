# Go frame intonation trainer

`go run ./cmd/tools/train-frame-intonation` trains the shipped frame TCN architecture with gograd v0.1.0 and exports version 8 JSON for `internal/prosody`. Model outputs and F0 caches must be under `out/`. Existing model JSON files are never overwritten.

## Japanese v10

The exact v10 dataset is `out/mfa-align-20261002/base-mfa.jsonl`, SHA-256 `12218686bb4ce92f51dca69b31df0ed850e78075fef3c2f015e8aba1ea7397a3`. FNV-1a ID splitting gives 558 train, 72 validation, and 70 test records. Timed tokens contain MFA vowel-onset note intervals and Open JTalk annotations. The trainer uses internal autocorrelation F0, 10 ms frames, 40 ms log-F0 smoothing, ±250 cent targets divided by 250, smooth L1 frame and adjacent-frame losses with delta weight 0.35, AdamW at 0.002 with 1e-5 decay, gradient clipping at 1, and a 32-channel residual TCN with dilations 1, 2, 4, 8, 16, and 32. It selects the best of 24 epochs by validation rendered contour MAE.

```powershell
go run ./cmd/tools/train-frame-intonation `
  --dataset out/mfa-align-20261002/base-mfa.jsonl `
  --training-corpus "Tsukuyomi-chan Corpus Vol.1 + Minnade JSUT basic5000" `
  --license-notice licenses/TSUKUYOMI-CORPUS.txt `
  --license-notice licenses/MINNADE-JSUT-CORPUS.txt `
  --license-notice licenses/MFA-Japanese-NOTICE.txt `
  --device cpu --epochs 24 --hidden 32 --batch-size 32 `
  --dilations 1,2,4,8,16,32 `
  --f0-cache out/mfa-align-20261002/f0-internal `
  --no-openjtalk-accent `
  --out out/frame-intonation-go/v10-candidate.json
```

## English v1

The exact English dataset is `out/english-frame-v1/corpus.jsonl`, SHA-256 `f061fc94739b58591e7827567cbe62d1edd04fc52d8b1fb0d618ea17352414a5`. Its speaker splits contain 554 train, 83 validation, and 101 test records. Timed tokens contain aligned ARPABET phones and stress. The shipped model used WORLD Harvest, ±400 cent targets, 0.65 render strength, 200 cent p99, 250 cent maximum, 40 epochs, width 32, batch size 16, and dilations 1, 2, 4, and 8. The Go command applies these as English defaults.

```powershell
python tools/cache-frame-f0.py --dataset out/english-frame-v1/corpus.jsonl `
  --cache out/frame-intonation-go/english-f0-cache `
  --world-engine runtime/utautts-world-engine.dll --workers 8
go run ./cmd/tools/train-frame-intonation `
  --dataset out/english-frame-v1/corpus.jsonl --language en `
  --f0-source world --world-engine runtime/utautts-world-engine.dll `
  --f0-cache out/frame-intonation-go/english-f0-cache `
  --training-corpus "LibriTTS-R train_clean_100 pilot (24 speakers; selected utterances)" `
  --model-license "CC BY 4.0 (model weights)" `
  --license-notice licenses/ENGLISH-FRAME-INTONATION-TCN-V1.txt `
  --license-notice licenses/LibriTTS-R-NOTICE.txt `
  --license-notice licenses/MFA-English-ARPA-NOTICE.txt `
  --license-notice licenses/CC-BY-4.0.txt `
  --device cpu --epochs 40 --hidden 32 --batch-size 16 `
  --dilations 1,2,4,8 --out out/frame-intonation-go/en-candidate.json
```

Input is version-1 JSONL with IDs, audio paths, text, and timed tokens. By default, Japanese text is reanalyzed through the same Open JTalk helper used by `internal/openjtalk`; pass `--openjtalk-features` and `--openjtalk-dictionary` when the helper and dictionary are outside `runtime/`. Tokens with a different mora or pause sequence are excluded, and training requires at least 60% alignment. `--no-openjtalk-accent` uses the already aligned corpus annotations, as in the measured v10 reproduction above. English tokens need aligned phones, word indices, stress, speakers, and explicit splits. The command reads and writes the Python trainer's SHA-1 keyed `.npy` F0 cache. Missing Japanese entries use Go autocorrelation; missing English entries use the Windows WORLD DLL. Other platforms can read a prefilled WORLD cache.

Go and PyTorch use different random streams for initialization and shuffling, so weights are not bitwise identical. `--index-only` prints the dataset hash, feature count, and split sizes without F0 extraction. `--features-only` prepares targets and exits. `--all-data-training` includes validation and test IDs in Japanese training and marks metrics as in-sample; English speaker splits reject it. `--predict-corpus` and `--predict-out` export centered, bounded frame curves from JSON or JSONL cases, including raw Japanese text through Open JTalk with deterministic fallback. English exports include validation and test flat-baseline metrics. `--audio-root`, `--description`, and repeatable `--recommended-renderer` are available; `--f0-method` accepts Harvest (1). PyTorch `--compile` has no Go equivalent or Go flag. gograd supports CPU and CUDA devices here; PyTorch's XPU, MPS, and numbered CUDA device selectors are not available. Renderer settings can be overridden with `--render-strength`, `--render-smoothing-ms`, `--render-p99-cents`, and `--render-max-cents`.

UtauTTS uses gograd v0.1.1, which bounds the CUDA buffer cache for variable sequence lengths; the earlier out-of-memory failure on an 8 GB GPU does not occur. On CUDA, the command prints gograd allocator live, cached, reserved, and peak bytes after each epoch.
