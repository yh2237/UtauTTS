# フレーム抑揚モデルの学習

## 準備

Python環境にはPyTorch、NumPy、pyopenjtalkが必要です。`--f0-source internal`では追加の実行ファイルは不要です。`--f0-source world`を使う場合は独自WORLDエンジンが必要です。ビルドスクリプトは、Windowsが`tools/build-world-engine.ps1`、Linuxが`tools/build-world-engine.sh`、macOSが`tools/build-world-engine-macos.sh`です。

学習データは`version: 1`のJSONLです。レコードは`id`、`audio_path`、`tokens`、`text`を持ちます。学習元コーパスの`--training-corpus`、`--training-corpus-license`、`--model-license`、`--license-notice`、`--source-notice`は必須です。スキーマは[技術設計ガイド](technical-design.md)を参照してください。

## 学習オプション

`--holdout-test`はIDの固定ハッシュで学習・検証・テストを分離します。`--all-data-training`を指定すると監視用の文も学習に含めます。この場合の指標は学習データ内の性能を示します。未知文の候補比較には分離データを使います。`--target-smooth-ms`はF0教師の平滑化幅、`--delta-weight`は隣接フレーム差分損失の重みで、どちらも大きくすると抑揚が滑らかになります。`--f0-cache`は抽出したF0を記録し、再実行を高速化します。

各epochで生のピッチ誤差と再生時の処理を反映した誤差を記録します。後者は予測と教師の両方に平滑化・強度・振幅制限を適用した値です。この値が最小の重みを保存します。音声の自然さは別途聴取で評価します。

## ITA Corpus Rionでの学習

ITA Corpus Rionは、50話者がITA Corpus Emotionの同じ100文を読んだ48 kHz/24 bitの音声コーパスです。音声と同じフォルダーに、公式ITA Corpusから取得した emotion_transcript_utf8.txt を配置します。同じ文を読む全話者は、学習・検証のどちらか片方にだけ入ります。

コーパスには音素時刻がないため、まず`--alignment viterbi`を使います。これはOpen JTalkのアクセント注釈を弱い音響モデルとして、有声フレームがそのモーラの高低に近づくよう、モーラ長の上下限付きViterbiで境界を推定します。単純なDTWと違い、各モーラが妥当な長さに収まるため退化した経路になりません。

~~~powershell
python tools/prepare-ita-corpus-rion.py --corpus data/ita-corpus-rion --out out/ita-corpus-rion-frame.jsonl `
  --alignment viterbi --world-engine runtime/utautts-world-engine.dll --require-reading-match `
  --lower-duration-factor 0.45 --upper-duration-factor 2.20
python tools/train-frame-intonation-tcn.py --dataset out/ita-corpus-rion-frame.jsonl `
  --f0-source world --world-engine runtime/utautts-world-engine.dll `
  --f0-cache out/f0-cache-world --device cuda --holdout-test --epochs 24 --hidden 32 --batch-size 64 `
  --model-id frame-intonation-v9-ita-corpus-rion --display-name "Frame intonation TCN v9 ITA Corpus Rion" `
  --description "ITA Corpus RionのEmotion音声で学習したフレーム抑揚モデル" `
  --training-corpus "ITA Corpus Rion (Emotion)" `
  --training-corpus-license "CC BY 4.0; source page additionally prohibits resale of the audio dataset" `
  --model-license "MIT License" --license-notice licenses/ITA-CORPUS-RION.txt --source-notice licenses/ITA-CORPUS-RION.txt `
  --out out/frame-intonation-v9-ita-corpus-rion.json
~~~

`--alignment viterbi`はアクセント注釈を使った強制アラインメントです。`--world-engine`を付けるとWORLD Harvestで推定し、省略すると高速な内蔵自己相関F0を使います。`--require-reading-match`は公式読みとモーラ数が一致しない誤読を除外します。`--alignment energy`は均等配置の境界を近傍のエネルギー谷へ寄せるだけで、効果は限定的です。`--f0-source world`はv8と同じWORLD HarvestでF0教師を作り、`--f0-cache`は抽出したF0を記録して再実行を高速化します。

### ITA Corpus Rionでの結果（2026-09-22）

女性25話者（F1-F25）で、アラインメント手法による差を比較しました。

| 手法 | 検証raw MAE | 検証rendered MAE | test raw | test rendered | 採用epoch |
| --- | --- | --- | --- | --- | --- |
| 均等配置 + internal F0 | 151.7 | 42.1 | 151.0 | 42.4 | 3 |
| 均等配置 + WORLD F0 + 母音特徴 | 153.0 | 43.1 | 154.8 | 43.6 | 3 |
| **アクセントViterbi + WORLD F0** | **113.6** | **31.4** | **108.2** | **29.7** | **13** |

均等配置では採用epochが3で早期に頭打ちになり、未学習文へ汎化しませんでした。アクセントViterbiでは採用epochが13まで伸び、rendered MAEは26〜31程度になりました。アクセント注釈を使うため教師信号は完全な自然発話の記録ではありませんが、音素時刻のないコーパスでモーラ境界を妥当に推定し、未学習文への汎化を改善します。

## Kokoro Speech Datasetでの学習

Kokoro Speech Datasetは、単一話者が青空文庫の小説を朗読したパブリックドメインの音声です。`metadata.csv`と`wavs/`を用意し、コーパスと同じViterbiアラインメントで学習します。

~~~powershell
python tools/prepare-kokoro-frame-data.py --corpus data/kokoro --out out/kokoro-frame.jsonl `
  --alignment viterbi --allow-reading-mismatch
python tools/train-frame-intonation-tcn.py --dataset out/kokoro-frame.jsonl `
  --f0-source internal --device cuda --holdout-test --epochs 24 --hidden 32 --batch-size 64 `
  --target-smooth-ms 80 --delta-weight 0.6 `
  --training-corpus "Kokoro Speech Dataset v1.3" --training-corpus-license "Public domain" `
  --model-license "MIT License" --license-notice licenses/KOKORO-SPEECH-DATASET.txt --source-notice licenses/KOKORO-SPEECH-DATASET.txt `
  --out out/frame-intonation-v9-k.json
~~~

`--allow-reading-mismatch`は、コーパスのローマ字読みとOpen JTalkの読みが促音・長音の表記で異なるクリップも残します。読み検証を厳しくしたい場合は外してください。同梱の`frame-intonation-v9-k`はKokoro speech small（3冊、9,199文、単一話者）で学習し、検証rendered MAE 29.2でした。

## つくよみちゃんコーパスでの学習

つくよみちゃんコーパス Vol.1は、単一話者が声優統計／JVSコーパスの100文を読んだ96 kHz float WAVです。PCM（16 bit、48 kHzなど）へ変換し、`metadata.csv`と`wavs/`を用意します。前処理は`prepare-kokoro-frame-data.py`を共用します。ライセンスは[つくよみちゃんコーパスの通知](../licenses/TSUKUYOMI-CORPUS.txt)を確認してください。

~~~powershell
python tools/prepare-kokoro-frame-data.py --corpus data/tsukuyomi --out out/tsukuyomi-frame.jsonl `
  --alignment viterbi --allow-reading-mismatch
python tools/train-frame-intonation-tcn.py --dataset out/tsukuyomi-frame.jsonl `
  --f0-source internal --device cuda --holdout-test --epochs 24 --hidden 32 --batch-size 32 `
  --target-smooth-ms 80 --delta-weight 0.6 `
  --training-corpus "Tsukuyomi-chan Corpus Vol.1 (Voice Actress 100)" `
  --training-corpus-license "Commercial use permitted with credit" `
  --model-license "MIT License" --license-notice licenses/TSUKUYOMI-CORPUS.txt --source-notice licenses/TSUKUYOMI-CORPUS.txt `
  --out out/frame-intonation-v9-t.json
~~~

`prepare-kokoro-frame-data.py`は`id|text|reading`の`metadata.csv`と`wavs/<id>.wav`を読みます。つくよみちゃんコーパスは100文と少ないため、同梱モデルは同じ話者の追加データ（夢前黎の音声寄せ集めなど）で安定します。

## 聴取比較

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/japanese-bank" --renderers utautts-world-phrase --model-file out/frame-intonation-candidate.json --repeat 1 --out out/candidate-listening
```

比較条件: 既定の`frame-intonation-v9-t`と同じ文章・音源・Renderer・設定。確認項目: 合成計画の原音選択と時間配置。採用判断: 学習にない文章の試聴。評価項目: [読み上げ品質の評価](../tools/evaluation/README.md)。

## ツールの役割

| ツール | 用途 |
| --- | --- |
| `prepare-ita-corpus-rion.py` | ITA Corpus Rionを学習用JSONLへまとめる |
| `prepare-kokoro-frame-data.py` | Kokoro／つくよみちゃんコーパスのクリップを学習用JSONLへまとめる |
| `mora_alignment.py` | 音素時刻のないコーパス向けアクセントViterbiアラインメント |
| `train-frame-intonation-tcn.py` | フレーム抑揚モデルの学習と予測 |
| `frame_render_metrics.py` | 再生時のピッチ処理を反映した評価 |
| `cmd/tools/tts-eval` | 合成音声と原音選択の比較 |

これらはモデルの世代に依存しない共通ツールです。候補ごとの設定は実行引数と出力先で区別します。
