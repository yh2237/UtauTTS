# モデルの学習

抑揚モデルおよび時間伸縮モデルの学習手順です。学習器およびデータ処理コマンドは `cmd/tools/` 配下に配置されています（Goトレーナーには `gograd` を使用）。

モデルの配布ライセンスおよびデータ出典については [モデル一覧](../models/README.md) を参照してください。

---

## 1. 日本語フレーム抑揚モデル（v12）

現行の日本語標準モデル `frame-intonation-tcn-v12` のF0ヘッドは、Irodori-TTS音声および各種コーパスを用いて学習されています。

### 学習手順の概要
1. コーパスのテキストと音声からMFA整列を行い、音素境界時刻を確定します。
2. WORLD HarvestによりF0を抽出し、オクターブ誤推定の補正と平滑化を行って目標F0データを作成します。
3. `train-speech-timing` コマンドでTCNを学習します。
   ```powershell
   go run ./cmd/tools/train-speech-timing `
     --dataset out/irodori-teacher/train-ird.jsonl `
     --alignments "out/mfa-align/alignments,out/irodori-teacher/alignments-ird" `
     --cache out/irodori-teacher/features-ird.gob `
     --language ja --f0 --f0-teacher out/irodori-teacher/teacher-f0.jsonl `
     --valid 300 --steps 24000 --window 1000 --batch-size 8 `
     --f0-dilations "1 2 4 8 16 32 64 1 2 4 8 16 32 64" `
     --plan-augment `
     --out out/irodori-teacher/f0-v12.safetensors
   ```
4. 学習した重みを既存のベースモデルと結合してモデルJSONを出力します。
   ```powershell
   go run ./cmd/tools/package-f0-model `
     --weights out/irodori-teacher/f0-v12.safetensors `
     --base models/frame-intonation-tcn-v12.json `
     --id my-f0-v12 `
     --display-name "My Custom F0 Model" `
     --natural-scale --base-blend 0 --pitch-offset-cents -50 `
     --out out/my-f0-v12.json
   ```

`--f0-teacher` には `id` と `cents` を持つJSONLを渡します。教師F0はWORLD Harvest（10ms、80〜800Hz）で抽出し、最大から−40dB以下のフレームを除外します。前後150msの中央値から600cent以上離れた値をオクターブ単位で折り返し、σ20msで平滑化して、発話の中央値を0にします。ローダーは値を100で割るため、自然スケール（log/0.3）に合わせて `cents` には「セント×100/519.3」を記録します。モデルJSONへの変換では `--natural-scale` を指定します。

- `--plan-augment`: 学習発話を合成時と同じ一定のモーラ長（120ms±15、休止180ms）へ並べ直し、モーラごとにF0目標を伸縮した複製を学習へ追加します。同梱モデルの検証では、プラン時間でのF0相関が0.59から0.75へ改善しました。
- `--f0-dilations`: 受容野を広げ、文全体の抑揚の流れを学習させます。

---

## 2. 英語フレーム抑揚モデル

`frame-intonation-tcn-en-v1` は、LibriTTS-Rの音声データとMFAによるARPAbet音素アライメント・強勢情報を用いて学習します。

```powershell
go run ./cmd/tools/train-frame-intonation `
  --dataset out/english-frame/corpus.jsonl `
  --language en `
  --f0-source world `
  --training-corpus "LibriTTS-R train_clean_100 subset" `
  --model-license "CC BY 4.0" `
  --license-notice licenses/LibriTTS-R-NOTICE.txt `
  --license-notice licenses/MFA-English-ARPA-NOTICE.txt `
  --epochs 40 --hidden 32 --batch-size 16 `
  --out out/english-frame/candidate.json
```

---

## 3. 中国語声調抑揚モデル

`tone-intonation-zh-v1` は、AISHELL-3の音声とPaddleSpeechの音節境界情報を使用し、声調の基本曲線に対する文脈補正値をRidge回帰によって学習します。

```powershell
go run ./cmd/tools/train-mandarin-intonation `
  --parquet data/aishell3/train.parquet `
  --alignments data/aishell3/alignments `
  --limit-per-speaker 250 `
  --out out/tone-intonation-zh-v1/candidate.json
```

---

## 4. 時間伸縮目標モデル（Speech Timing Target）

モーラ内の自然な子音・母音の伸縮比率を決定するモデルです。言語ごとに訓練した重み（`.safetensors`）をWORLD bridgeに内包して使用します。
