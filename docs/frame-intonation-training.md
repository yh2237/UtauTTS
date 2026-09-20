# フレーム抑揚モデルの学習

## Kokoro Speech Datasetで正式版を作る

正式版では、均等配置した時刻をそのまま学習へ渡しません。公式メタデータの区間を音声から切り出し、Open JTalkでモーラ情報を作った後、公式Kokoro-AlignのCTCチェックポイントで音素境界を推定します。`prepare-kokoro-frame-data.py`の出力は、この処理に渡す暫定データです。

前提として、Kokoro Speech Dataset 1.3のメタデータをdata/kokoro-metadataへ、対応する原音声をdata/kokoro-full/sourceへ配置します。Kokoro-Alignの公式リポジトリとctc-20221201/ctc-last.pthもdata/kokoro-alignへ配置します。音声の取得・ライセンス条件はlicenses/KOKORO-SPEECH-DATASET.txtを確認してください。

~~~powershell
python tools/extract-kokoro-clips.py --metadata data/kokoro-metadata --source-root data/kokoro-full/source --out data/kokoro-full-wav
python tools/prepare-kokoro-frame-data.py --corpus data/kokoro-full-wav --out out/kokoro-full-uniform-frame.jsonl
python tools/align-kokoro-frame-data.py --input out/kokoro-full-uniform-frame.jsonl --out out/kokoro-formal-frame.jsonl --checkpoint data/kokoro-align/model/ctc-20221201/ctc-last.pth --method viterbi
~~~

正式モデルの学習例です。`--f0-source internal`は高速な自己相関F0抽出を使います。WORLD Harvestと比較する場合は、`--f0-source world`と`--world-engine`を指定します。使用したF0抽出器は、モデルJSONの`training.f0_source`へ記録されます。

~~~powershell
python tools/train-frame-intonation-tcn.py --dataset out/kokoro-formal-frame.jsonl --dataset-kind generic --f0-source internal --holdout-test --epochs 3 --hidden 32 --batch-size 64 --model-id frame-intonation-v9-kokoro --display-name "Frame intonation TCN v9 Kokoro" --description "Kokoro Speech Datasetと公式Kokoro-Align CTC時刻で学習したフレーム抑揚モデル" --training-corpus "Kokoro Speech Dataset 1.3" --training-corpus-license "Public-domain statement in the Kokoro Speech Dataset README; jurisdiction should be verified before redistribution" --model-license "MIT License" --license-notice licenses/KOKORO-SPEECH-DATASET.txt --source-notice licenses/KOKORO-SPEECH-DATASET.txt --out out/frame-intonation-v9-kokoro.json
~~~

学習済みモデルの作成と評価に必要な手順を記載します。

`tools/train-frame-intonation-tcn.py`は、学習用JSONLと対応する音声からフレーム単位のピッチを予測するTCNを学習します。出力はversion 8形式のJSONです。モデル名の番号とJSONのスキーマ番号は別です。別のコーパスで学習する場合は、出力JSONのライセンスと出典を対象コーパスの条件に合わせます。

## 準備

Python環境にはPyTorch、NumPy、pyopenjtalkが必要です。`--f0-source internal`では追加の実行ファイルは不要です。`--f0-source world`を使う場合は独自WORLDエンジンが必要です。ビルドスクリプトは、Windowsが`tools/build-world-engine.ps1`、Linuxが`tools/build-world-engine.sh`、macOSが`tools/build-world-engine-macos.sh`です。

JSUTで学習する場合は、JSUT BASIC5000の音声、`transcript_utf8.txt`、sarulab-speech/jsut-labelの対応ラベルが必要です。利用条件は[音声とラベルの通知](../licenses/JSUT-DATA-AND-LABELS.txt)を確認してください。

## JSUTでの学習と評価

コマンド例の入力: 展開したJSUTのパス。生成先: `out/`。モデル一覧への登録先: `models/`。

```powershell
python tools/prepare-jsut-full-labels.py --labels "./data/jsut-label" --corpus "./data/jsut/basic5000" --out out/jsut-all5000.jsonl
python tools/train-frame-intonation-tcn.py --dataset out/jsut-all5000.jsonl --world-engine runtime/utautts-world-engine.dll --holdout-test --model-id frame-intonation-candidate --display-name "Frame intonation candidate" --out out/frame-intonation-candidate.json
```

`--holdout-test`はIDの固定ハッシュで学習・検証・テストを分離します。`--all-data-training`を指定すると監視用の文も学習に含めます。この場合の指標は学習データ内の性能を示します。未知文の候補比較には分離データを使います。

上の例は、同梱の`frame-intonation-v8`と同じくOpen JTalkのアクセント・品詞特徴を使う条件です。`--jsut-context-labels`を追加すると、jsut-labelのアクセント注釈を使う別条件の候補になります。推論時の品詞と語境界は未知値として扱うため、Open JTalk特徴とは別条件です。

各epochで生のピッチ誤差と再生時の処理を反映した誤差を記録します。後者は予測と教師の両方に平滑化・強度・振幅制限を適用した値です。この値が最小の重みを保存します。音声の自然さは別途聴取で評価します。

## 聴取比較

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/japanese-bank" --renderers utautts-world-phrase --model-file out/frame-intonation-candidate.json --repeat 1 --out out/candidate-listening
```

比較条件: 既定の`frame-intonation-v8`と同じ文章・音源・Renderer・設定。確認項目: 合成計画の原音選択と時間配置。採用判断: 学習にない文章の試聴。評価項目: [読み上げ品質の評価](../tools/evaluation/README.md)。

## ツールの役割

| ツール | 用途 |
| --- | --- |
| `prepare-jsut-full-labels.py` | 音声とラベルを学習用JSONLへまとめる |
| `train-frame-intonation-tcn.py` | フレーム抑揚モデルの学習と予測 |
| `frame_render_metrics.py` | 再生時のピッチ処理を反映した評価 |
| `cmd/tools/tts-eval` | 合成音声と原音選択の比較 |

これらはモデルの世代に依存しない共通ツールです。`train-prosody-multitask.py`もフレームTCNの共通処理を使います。候補ごとの設定は実行引数と出力先で区別します。

## 均等配置を使う比較

Kokoroは文単位の音声を提供しますが、モーラ単位の時刻ラベルは提供しません。`prepare-kokoro-frame-data.py`は音声の有効区間を検出し、Open JTalkのモーラを区間内へ均等に配置します。これは時刻推定の影響を比較するための近似です。配布候補を作る場合は、冒頭の手順で強制アラインメントを使用してください。

```powershell
python tools/prepare-kokoro-frame-data.py --corpus ./data/kokoro --out ./out/kokoro-frame.jsonl
python tools/train-frame-intonation-tcn.py --dataset ./out/kokoro-frame.jsonl --dataset-kind generic --world-engine ./runtime/utautts-world-engine.dll --holdout-test --model-id frame-intonation-v9 --display-name "Frame intonation v9" --training-corpus "Kokoro Speech Dataset 1.3" --training-corpus-license "Public-domain statement in the Kokoro Speech Dataset README; jurisdiction should be verified before redistribution" --model-license "MIT License" --license-notice licenses/KOKORO-SPEECH-DATASET.txt --source-notice licenses/KOKORO-SPEECH-DATASET.txt --out ./out/frame-intonation-v9.json
```

入力音声はモノラル16ビットPCM WAVを使います。本文中の空白は単語境界であり、休止として扱いません。公式メタデータの読みとOpen JTalkの音素列が異なる発話は既定で除外します。`--allow-reading-mismatch`は原因調査専用です。モデルJSONにはコーパス名、利用条件、通知ファイルを必ず記録します。
