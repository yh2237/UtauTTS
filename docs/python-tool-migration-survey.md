# Python workflow survey (2026-10-04)

Inventory: 41 Python files in `tools/`, two in `qt/wasm/`, and one in `web/`. `tools/intonation-lab/` and `tools/evaluation/` contain no Python files. Categories: **A** uses PyTorch; **B** imports `train-frame-intonation-tcn.py`; **C** prepares or transforms training data; **D** bridges an external Python tool or MFA workflow; **E** release, license, or build helper; **N** other numerical training or analysis; **T** test. A file can have multiple categories. “Current” means a documented command, its imported helper, or its companion test; “optional” is documented but not shipped as a model.

| Python file | Class | Docs and script references | Current use / migration disposition |
| --- | --- | --- | --- |
| `tools/align-intonation-mfa.py` | D | `docs/frame-intonation-training.md`; `prepare-intonation-frame-data.py`, `train-mora-duration-tcn.py` | Yes; MFA prepare/import bridge remains Python |
| `tools/cache-frame-f0.py` | B,C | `docs/go-frame-intonation-training.md`, `docs/english-frame-intonation-training.md` | Go cache command verified on three English records |
| `tools/collect-pyinstaller-runtime-licenses.py` | E | `tools/build-linux.sh`, `build-macos.sh`, `build-release.ps1` | Yes; release helper stays Python |
| `tools/copy-model-license-notices.py` | E | Linux/macOS/Windows build and package-test scripts | Yes; release helper stays Python |
| `tools/frame_render_metrics.py` | N | `docs/frame-intonation-training.md`; `train-frame-intonation-tcn.py`, its test | Yes; Go trainer already has renderer metrics |
| `tools/import-cmudict.py` | E | No filename reference found | Build-time dictionary import; outside training migration |
| `tools/mora_alignment.py` | B,C | `docs/frame-intonation-training.md`; `prepare-intonation-frame-data.py` | Viterbi path remains Python; documentation says it is unsuitable for training, so propose retiring it with the removed trial trainer |
| `tools/openjtalk-feature-bridge.py` | D,E | Open JTalk bridge build scripts | Yes; Python packaging bridge remains |
| `tools/openjtalk_feature_common.py` | C,D | Open JTalk bridge build script; `openjtalk_features.py`, bridge | Yes; equivalent runtime analysis exists in `internal/openjtalk` |
| `tools/openjtalk_features.py` | C,D | Open JTalk bridge build script; Japanese data preparation | Yes; equivalent runtime helper exists in `internal/openjtalk` |
| `tools/performance-baseline.py` | E | No filename reference found | Optional benchmark helper |
| `tools/plot-source-analysis.py` | N | `docs/source-understanding.md` | Yes; review plot, outside data preparation |
| `tools/prepare-english-frame-data.py` | C | `docs/english-frame-intonation-training.md` | Go command verified on 738 local MFA records |
| `tools/prepare-intonation-frame-data.py` | C,D | `docs/frame-intonation-training.md`; `align-intonation-mfa.py` | Go uniform-to-MFA path verified on two JSUT utterances; G2P phone provenance and Viterbi remain Python |
| `tools/prepare-libritts-r-subset.py` | C | `docs/english-frame-intonation-training.md` | Go command verified on two real utterances and WAV hashes |
| `tools/prepare-minnade-jsut.py` | C | `docs/frame-intonation-training.md` | Go command verified byte for byte on two real records |
| `tools/prepare-multilingual-speech.py` | C | `docs/multilingual-learning.md`; `test_multilingual_speech.py` | Go command verified on a synthetic tone and one clipped JSUT recording |
| `tools/source-phone-alignment.py` | D | `docs/source-understanding.md`; source-span tools and test | Yes; MFA prepare/import/audit workflow remains Python |
| `tools/source-phone-discovery.py` | D | `docs/source-understanding.md`; source discovery test | Yes; MFA candidate workflow remains Python |
| `tools/source-phone-discovery-zh.py` | D | `docs/source-understanding.md`; Chinese discovery test | Yes; MFA candidate workflow remains Python |
| `tools/source-span-auto.py` | C,D | `docs/source-understanding.md`; discovery tools and span-auto test | Go `build` subcommand verified on five local sources; `map` still depends on Python span proposal/selection and MFA audit |
| `tools/source-span-mapping.py` | C,D | `docs/source-understanding.md`; span-auto and mapping test | Yes; coupled to Python MFA/source-analysis workflow |
| `tools/source_phone_common.py` | C,D | `docs/synthesis-architecture.md`; source phone tools | Yes; shared MFA/source-analysis helper |
| `tools/test-source-phone-alignment.py` | T,D | Companion test | Yes; Python MFA workflow test |
| `tools/test-source-phone-discovery.py` | T,D | Companion test | Yes; Python MFA workflow test |
| `tools/test-source-phone-discovery-zh.py` | T,D | Companion test | Yes; Python MFA workflow test |
| `tools/test-source-span-auto.py` | T,D | Companion test | Yes; Python MFA workflow test |
| `tools/test-source-span-mapping.py` | T,D | Companion test | Yes; Python MFA workflow test |
| `tools/test_english_frame_data.py` | B,T | `prepare-english-frame-data.py` and trainer via dynamic imports | Yes; replace with Go parity tests |
| `tools/test_frame_render_metrics.py` | T | `frame_render_metrics.py` | Yes; Go trainer already tests rendering |
| `tools/test_multilingual_speech.py` | T | Multilingual prepare/train scripts | Yes; companion test |
| `tools/test_world_engine_f0.py` | T | `world_engine_f0.py` | Yes; Go trainer has WORLD parity test |
| `tools/torch_device.py` | A | `train-frame-intonation-tcn.py` | Yes until Python trainer dependents are removed |
| `tools/train-frame-intonation-tcn.py` | A | Frame and English training docs; five dynamic importers | Go trainer exists; retain until importers migrate |
| `tools/train-manual-intonation-residual.py` | A,B | `docs/frame-intonation-training.md` | Go trainer implemented; 44 fixture phrases and 146 features match Python, JSON loads and predicts; real saved Lab session unavailable |
| `tools/train-mandarin-intonation.py` | N | `docs/mandarin-intonation-training.md` | Yes; NumPy linear fit, outside PyTorch classes A-C |
| `tools/train-mora-duration-tcn.py` | A,B | No current doc; imports MFA bridge | Removed trial model; propose deletion, do not port |
| `tools/train-multilingual-speech.py` | N | `docs/multilingual-learning.md`; companion test | Yes; NumPy ridge trainer, outside PyTorch classes A-C |
| `tools/verify-openjtalk-feature-bridge.py` | E | Open JTalk bridge build scripts | Yes; build verification |
| `tools/verify-qt-sbom.py` | E | Release and macOS build scripts | Yes; release verification |
| `tools/world_engine_f0.py` | C | `train-frame-intonation-tcn.py`, `train-mandarin-intonation.py`, test | Yes; Go trainer already has WORLD Harvest path |
| `qt/wasm/cloudflare.py` | E | `web/test-cloudflare-browser.cjs`, companion test | Yes; deployment helper |
| `qt/wasm/test_cloudflare.py` | T,E | `cloudflare.py` | Yes; deployment test |
| `web/build-voice.py` | E | `web/build.ps1`, `web/build.sh` | Yes; website build helper |

No `models/*.json` residual model exists. `docs/frame-intonation-training.md` explicitly demonstrates an output under `out/`, and `internal/prosody` parses manual residual version 11. No current documentation or shipped model refers to the removed mora-duration trial.

## Verified Go commands in this round

| New command under `cmd/tools/` | Verification | Remaining limit |
| --- | --- | --- |
| `cache-frame-f0` | Three real English records: same cache keys and frame counts (660, 340, 772); maximum F0 difference 1.99e-13 Hz | Windows WORLD DLL for extraction; other OS targets compile but cannot extract with that DLL |
| `prepare-english-frame-data` | 738 real records structurally equal to Python; 554 train, 83 validation, 101 test | Two of 104 rejection *messages* differ only in file-not-found formatting |
| `prepare-intonation-frame-data` | Two real JSUT utterances, 61 tokens: readings and token fields equal; maximum timing difference 0 ms | Uniform route only; native helper omits pyopenjtalk.g2p phone stream, so `--allow-reading-mismatch` is required; Viterbi remains Python and is not used for training |
| `prepare-libritts-r-subset` | Two records from the local archive: transcript metadata and WAV SHA-256 equal | Larger extraction not repeated |
| `prepare-minnade-jsut` | Two real records: metadata and WAVs byte-identical | Larger extraction not repeated |
| `prepare-multilingual-speech` | One synthetic voiced tone and one clipped real JSUT WAV: JSON records equal to Python | No real multilingual observation/linguistic template pair was present |
| `source-span-auto build` | Five real selected source clips: JSON data equal to Python | `map` and `source-span-mapping propose/select` remain in Python with the MFA audit workflow |
| `train-manual-intonation-residual` | Eight fixture Lab entries: 44 phrases, 146 features, every feature and target within 1e-8 of Python; exported JSON loads through `internal/prosody` and predicts a contour; after 20 epochs the exported best-checkpoint validation MAE was 1.065 cents in Python and 1.381 cents in Go | No saved `.utautts` session was found; initialization and shuffle order differ, so weights and short-run metrics are not identical |

All Go commands require new output paths under `out/`. Example invocations:

```powershell
go run ./cmd/tools/prepare-minnade-jsut --root data/みんなで作るJSUTコーパスbasic5000 --out out/jsut
go run ./cmd/tools/prepare-intonation-frame-data --corpus out/jsut --alignment uniform --allow-reading-mismatch --out out/frame.jsonl
go run ./cmd/tools/prepare-libritts-r-subset --archive data/libritts-r/downloads/train_clean_100.tar.gz --out out/libritts-subset
go run ./cmd/tools/prepare-english-frame-data --manifest out/libritts-subset/manifest.json --alignments out/mfa-alignments --out out/english-frame.jsonl
go run ./cmd/tools/cache-frame-f0 --dataset out/english-frame.jsonl --cache out/frame-f0-cache --world-engine runtime/utautts-world-engine.dll
go run ./cmd/tools/prepare-multilingual-speech --observations out/observations.json --out out/multilingual.jsonl out/template.json
go run ./cmd/tools/source-span-auto build --spans out/source-spans/spans.json --out out/source-phone-library.json
go run ./cmd/tools/train-manual-intonation-residual --base-model models/frame-intonation-tcn-v10.json --out out/manual-residual.json out/lab-session.utautts
```

The example paths illustrate command shape; the optional Lab session and multilingual observation/template pair are absent locally.

## Python retirement and documentation follow-up

`train-mora-duration-tcn.py` is a removed trial with no shipped model or current documented command; propose deleting it. The Viterbi-only `mora_alignment.py` can be retired with that trial once the old Python Japanese preparer is removed. `train-manual-intonation-residual.py` has no shipped model, but its output format is used by `internal/prosody` and it is explicitly documented, so retain it until the Go trainer is checked on a real saved Lab session. The Python `train-frame-intonation-tcn.py` remains importable by the old cache, tests, mora alignment and residual script until those Python callers are retired; do not delete it yet.

After switching workflow documentation to Go, the Python copies of `cache-frame-f0.py`, `prepare-english-frame-data.py`, `prepare-libritts-r-subset.py`, `prepare-minnade-jsut.py`, and `prepare-multilingual-speech.py` are candidates for deletion. `prepare-intonation-frame-data.py` still supplies strict G2P provenance and optional Viterbi, while `source-span-auto.py` still supplies `map`; keep these Python files. The source phone alignment/discovery bridge and `source-span-mapping.py` remain part of the MFA workflow. NumPy trainers `train-mandarin-intonation.py` and `train-multilingual-speech.py` were outside the requested PyTorch/data-preparation classes and remain Python.

Existing docs requiring command updates are `docs/frame-intonation-training.md`, `docs/go-frame-intonation-training.md`, `docs/english-frame-intonation-training.md`, `docs/multilingual-learning.md`, and `docs/source-understanding.md`. They were left untouched by this round. The release scripts that call the Python license/build helpers should continue doing so.
