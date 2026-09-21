# フレーム抑揚モデルの学習

## 準備

Python環境にはPyTorch、NumPy、pyopenjtalkが必要です。`--f0-source internal`では追加の実行ファイルは不要です。`--f0-source world`を使う場合は独自WORLDエンジンが必要です。ビルドスクリプトは、Windowsが`tools/build-world-engine.ps1`、Linuxが`tools/build-world-engine.sh`、macOSが`tools/build-world-engine-macos.sh`です。

JSUTで学習する場合は、JSUT BASIC5000の音声、`transcript_utf8.txt`、sarulab-speech/jsut-labelの対応ラベルが必要です。利用条件は[音声とラベルの通知](../licenses/JSUT-DATA-AND-LABELS.txt)を確認してください。

## ITA Corpus Rionでの学習

ITA Corpus Rionは、50話者がITA Corpus Emotionの同じ100文を読んだ48 kHz/24 bitの音声コーパスです。音声と同じフォルダーに、公式ITA Corpusから取得した emotion_transcript_utf8.txt を配置します。前処理は音声の有効区間へOpen JTalkのモーラを均等配置する近似であり、強制アラインメントではありません。同じ文を読む全話者は、学習・検証のどちらか片方にだけ入ります。

~~~powershell
python tools/prepare-ita-corpus-rion.py --corpus data/ita-corpus-rion --out out/ita-corpus-rion-frame.jsonl
python tools/train-frame-intonation-tcn.py --dataset out/ita-corpus-rion-frame.jsonl --dataset-kind generic --f0-source internal --device cuda --holdout-test --epochs 6 --hidden 32 --batch-size 64 --model-id frame-intonation-v9-ita-corpus-rion --display-name "Frame intonation TCN v9 ITA Corpus Rion" --description "ITA Corpus RionのEmotion音声で学習したフレーム抑揚モデル" --training-corpus "ITA Corpus Rion (Emotion)" --training-corpus-license "CC BY 4.0; source page additionally prohibits resale of the audio dataset" --model-license "MIT License" --license-notice licenses/ITA-CORPUS-RION.txt --source-notice licenses/ITA-CORPUS-RION.txt --out out/frame-intonation-v9-ita-corpus-rion.json
~~~
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
