# Go speech timing target trainer

On Windows, train the version-1 speech timing target with gograd and the UtauTTS WORLD engine:

```powershell
go run ./cmd/tools/train-speech-timing --dataset out/mfa-align-20261002/base-mfa.jsonl --alignments out/mfa-align-20261002/alignments --world-engine runtime/utautts-world-engine.dll
```

The command reads the JSONL `id` and `audio_path` fields and each `<id>.json` MFA phones tier. It extracts 10 ms WORLD frames, 80 log-mel bands, phone context, and four continuous inputs, then caches the features at `out/speech-timing-target/go-features.gob`. Delete that cache when the audio, alignments, or extraction code changes. It trains the gograd `NewSpeechTiming(4)` model for 6000 steps, evaluates 30 held-out utterances every 250 steps, and saves the best safetensors checkpoint and a parity fixture under `out/speech-timing-target/`. Output filenames include a timestamp; existing files are refused. Use `--out`, `--fixture`, `--cache`, `--steps`, `--valid`, `--seed`, and `--device` to override defaults.

Use `--features-only --features-json out/speech-timing-target/sample.json` to export the first three utterances for a feature comparison. Feature extraction requires the Windows WORLD DLL. A completed Go feature cache can be used for training on other platforms, subject to gograd device support. The validation split uses Go's seeded shuffle, so its score is a quality comparison with the Python run rather than a frame-for-frame identical held-out set.
