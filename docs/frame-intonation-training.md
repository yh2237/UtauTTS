# フレーム抑揚モデルの学習

`tools/train-frame-intonation-tcn.py`はフレーム単位のピッチを予測するTCNを学習します。出力はversion 8形式のJSONです。モデル名の番号とJSONのスキーマ番号は別です。

## 準備

Python環境にはPyTorch・NumPy・pyopenjtalkが必要です。F0抽出には独自WORLDエンジンを使います。`tools/build-world-engine.ps1`でビルドしてください。Linuxでは`tools/build-world-engine.sh`を使います。macOSでは`tools/build-world-engine-macos.sh`を使います。

JSUT BASIC5000の音声と`transcript_utf8.txt`に加えてsarulab-speech/jsut-labelの対応ラベルを用意します。利用条件は[音声とラベルの通知](../licenses/JSUT-DATA-AND-LABELS.txt)を確認してください。

## 学習と評価

次のパスは手元のデータに置き換えてください。候補モデルは`out/`へ出力します。同梱モデルの選択肢には自動で追加されません。

```powershell
python tools/prepare-jsut-full-labels.py --labels "./data/jsut-label" --corpus "./data/BASIC5000" --out out/jsut-all5000.jsonl
python tools/train-frame-intonation-tcn.py --dataset out/jsut-all5000.jsonl --world-engine runtime/utautts-world-engine.dll --jsut-context-labels --holdout-test --model-id frame-intonation-candidate --display-name "Frame intonation candidate" --out out/frame-intonation-candidate.json
```

`--holdout-test`はIDの固定ハッシュで学習・検証・テストを分離します。`--all-data-training`を指定すると監視用の文も学習に含まれます。この場合の指標は未知文での性能を示しません。候補の比較には分離したデータを使ってください。

`--jsut-context-labels`はJSUTのアクセント注釈を使います。品詞と語境界は未知として扱うため推論時のOpen JTalk特徴とは条件が異なります。

各epochで生のピッチ誤差と再生時の処理を反映した誤差を記録します。後者は予測と教師の両方に平滑化・強度・振幅制限を適用した値です。この値が最小の重みを保存します。音声の自然さを直接測る指標ではありません。

## 聴取比較

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/japanese-bank" --renderers utautts-world-phrase --model-file out/frame-intonation-candidate.json --repeat 1 --out out/candidate-listening
```

既定の`frame-intonation-v8`と同じ文章・音源・Renderer・設定で比較します。合成計画の原音選択と時間配置も確認してください。採用前には学習にない文章で試聴します。評価項目は[読み上げ品質の評価](../tools/evaluation/README.md)を参照してください。

## ツールの役割

| ツール | 用途 |
| --- | --- |
| `prepare-jsut-full-labels.py` | 音声とラベルを学習用JSONLへまとめる |
| `train-frame-intonation-tcn.py` | フレーム抑揚モデルの学習と予測 |
| `frame_render_metrics.py` | 再生時のピッチ処理を反映した評価 |
| `cmd/tools/tts-eval` | 合成音声と原音選択の比較 |

これらはモデルの世代に依存しない共通ツールです。`train-prosody-multitask.py`もフレームTCNの共通処理を使います。候補ごとの設定は実行引数と出力先で区別してください。
