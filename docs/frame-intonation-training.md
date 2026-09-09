# フレーム抑揚モデルの学習

`tools/train-frame-intonation-v9.py`はフレームTCNを学習し、互換性のあるversion 8形式のJSONを出力します。モデルIDのv9はスキーマ番号ではありません。出力を自動で同梱・既定設定にはしません。

Python環境にはPyTorch、NumPy、pyopenjtalkが必要です。F0抽出には`tools/build-world-engine.ps1`（Linuxは`.sh`、macOSは`build-world-engine-macos.sh`）で作る独自WORLDエンジンを使います。CPUでも実行できます。

## 全BASIC5000からの学習

JSUT BASIC5000の音声・transcript_utf8.txtと、sarulab-speech/jsut-labelの対応ラベルを用意します。音声・ラベルの利用条件は`licenses/JSUT-DATA-AND-LABELS.txt`を確認してください。

```powershell
python tools/prepare-jsut-full-labels.py --labels <label-directory> --corpus <basic5000-directory> --out out/jsut-all5000.jsonl
python tools/train-frame-intonation-v9.py --dataset out/jsut-all5000.jsonl --world-engine runtime/utautts-world-engine.dll --jsut-context-labels --all-data-training --out out/frame-intonation-v9.json
```

同梱v9はこの全件学習方式、hidden 32、24 epoch、seed 1、WORLDLINE Harvestで作成し、監視値が最良だった23 epoch目を採用しています。学習時はJSUTのアクセント注釈を使います。品詞と語境界は未知として扱い、推論時のOpen JTalk特徴との違いがある点に注意してください。

`--all-data-training`では5,000文すべてを学習します。監視用集合も学習に含まれ、指標は未知文での性能を示しません。JSONに`evaluation_is_in_sample: true`を記録します。この指定を省くと、IDの固定ハッシュで学習・検証・テストを分離します。

各epochで、生のピッチ誤差と、再生用の平滑化・強度・振幅制限を予測と教師の両方に適用した誤差を記録します。後者が最小の重みを保存します。この指標は音声の自然さや音質を直接測るものではありません。

## 聴取比較

```powershell
go run ./cmd/tools/tts-eval --voicebank <voicebank-directory> --renderers utautts-world-phrase --model-file out/frame-intonation-v9.json --repeat 1 --out out/v9-listening
```

比較対象も同じコーパス・音源・Renderer・設定で合成してください。Plan JSONの原音選択と時間配置を確認し、モデル以外の差を混ぜないようにします。採用前には学習にない文章でも試聴してください。同梱v8は既定として残し、v9は別の抑揚の選択肢として提供しています。
