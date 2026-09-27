# 英語フレーム抑揚モデルの学習

英語の音節特徴と音素時刻を使って、フレーム単位の抑揚モデルを学習します。日本語の学習手順は[フレーム抑揚モデルの学習](frame-intonation-training.md)、同梱モデルの出典と配布条件は[抑揚モデルのライセンス](../models/README.md)を参照してください。

## 準備

Python環境にPyTorchとNumPy、音素整列にMontreal Forced Aligner（MFA）が必要です。特徴抽出にはGoでビルドしたヘルパー、F0抽出には独自WORLDエンジンを使います。英語の前処理ではOpen JTalkを使いません。

## コーパスを準備する

LibriTTS-Rの`train_clean_100.tar.gz`と修復失敗リストを配布元から取得します。アーカイブは配布元の`md5sum.txt`で照合し、コーパス付属のライセンスと出典を保存してください。

次の例は24話者から各最大40発話を抽出します。話者数と発話数は学習に使うデータ量に合わせて変更してください。

```powershell
python tools/prepare-libritts-r-subset.py --archive data/libritts-r/downloads/train_clean_100.tar.gz --out data/libritts-r/subset --speakers 24 --per-speaker 40
```

## 音素を整列し、特徴を抽出する

MFAの`english_us_arpa`音響モデルと辞書を使用します。これらは前処理に必要で、UtauTTSの読み上げ時には不要です。

```powershell
mfa align data/libritts-r/subset english_us_arpa english_us_arpa out/english-frame/alignments --output_format json --num_jobs 4 --no_use_postgres
go build -o out/english-frame-data.exe ./cmd/tools/english-frame-data
python tools/prepare-english-frame-data.py --manifest data/libritts-r/subset/manifest.json --alignments out/english-frame/alignments --helper out/english-frame-data.exe --failed-list data/libritts-r/libritts_r_failed_speech_restoration_examples/train-clean-100_bad_sample_list.txt --out out/english-frame/corpus.jsonl
```

整列済みARPAbetを音節へ分け、音素列が一致する音節に実測区間を対応付けます。境界の均等分割は行いません。除外理由と話者単位の学習・検証・試験分割は、出力先の`corpus.audit.json`で確認できます。

## 学習する

WORLD Harvestで抽出したF0を、発話内中央値に対するセント値へ変換して学習します。F0キャッシュを作ってから学習すると、再実行時の抽出を省略できます。

```powershell
python tools/cache-frame-f0.py --dataset out/english-frame/corpus.jsonl --cache out/english-frame/f0-cache --world-engine runtime/utautts-world-engine.dll --workers 8
python tools/train-frame-intonation-tcn.py --dataset out/english-frame/corpus.jsonl `
  --language en --device cuda --epochs 40 --hidden 32 --batch-size 16 `
  --training-corpus "LibriTTS-R train_clean_100 subset" --model-license "CC BY 4.0" `
  --license-notice licenses/LibriTTS-R-NOTICE.txt --license-notice licenses/MFA-English-ARPA-NOTICE.txt `
  --model-id my-english-model-v1 --display-name "My English intonation model" `
  --world-engine runtime/utautts-world-engine.dll --f0-cache out/english-frame/f0-cache `
  --low-cents -400 --high-cents 400 --render-strength 0.65 --render-p99-cents 200 --render-max-cents 250 `
  --out out/english-frame/my-english-model-v1.json
```

モデルID・表示名・出力先は作成するモデルに合わせて変更してください。共通の学習オプションは[フレーム抑揚モデルの学習](frame-intonation-training.md#学習オプション)にあります。

## 合成して比較する

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/english-bank" --corpus tools/evaluation/english-v1.json --renderers utautts-world-phrase --model-file out/english-frame/my-english-model-v1.json --repeat 1 --out out/english-frame/listening
```

比較するモデル間で文章・音源・Renderer・設定を揃え、学習に使っていない文章を試聴します。学習時のピッチ誤差は教師輪郭との比較であり、音声の自然さを直接示す値ではありません。

この特徴抽出はDelta・VCCVの音節構成を対象としています。C+V・ARPAsingへの適用は未評価です。比較項目は[読み上げ品質の評価](../tools/evaluation/README.md)を参照してください。
