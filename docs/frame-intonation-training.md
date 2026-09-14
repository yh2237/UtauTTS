# フレーム抑揚モデルの学習

学習済みモデルの作成と評価に必要な手順を記載します。

`tools/train-frame-intonation-tcn.py`はJSUT BASIC5000の音声からフレーム単位のピッチを予測するTCNを学習します。出力はversion 8形式のJSONです。モデル名の番号とJSONのスキーマ番号は別です。このツールはJSUT用です。別のコーパスでは、出力JSONのライセンスと出典を対象コーパスの条件に合わせます。

## 準備

Python環境の依存: PyTorch・NumPy・pyopenjtalk。F0抽出: 独自WORLDエンジン。ビルドスクリプト: Windowsは`tools/build-world-engine.ps1`、Linuxは`tools/build-world-engine.sh`、macOSは`tools/build-world-engine-macos.sh`。

入力データ: JSUT BASIC5000の音声、`transcript_utf8.txt`、sarulab-speech/jsut-labelの対応ラベル。利用条件: [音声とラベルの通知](../licenses/JSUT-DATA-AND-LABELS.txt)。

## 学習と評価

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
