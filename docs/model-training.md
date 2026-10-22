# モデルの学習

抑揚モデルと時間伸縮モデルの学習の手順です。学習とデータ処理のコマンドは`cmd/tools/`にあり、Goの学習には`gograd`を使います。

モデルの配布ライセンスとデータの出典は[モデル一覧](../models/README.md)を参照してください。

---

## 1. 日本語フレーム抑揚モデル（v12）

日本語の標準モデル`frame-intonation-tcn-v12`のF0ヘッドは、Irodori-TTSの音声と各種のコーパスで学習しています。

### 手順
1. コーパスのテキストと音声をMFAで整列し、音素の境界の時刻を求めます。
2. WORLDのHarvestでF0を求め、オクターブの誤推定を直して平滑化し、目標のF0を作ります。
3. `train-speech-timing`でTCNを学習します。
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
4. 学習した重みを既存の基準モデルと合わせて、モデルJSONを出力します。
   ```powershell
   go run ./cmd/tools/package-f0-model `
     --weights out/irodori-teacher/f0-v12.safetensors `
     --base models/frame-intonation-tcn-v12.json `
     --id my-f0-v12 `
     --display-name "My Custom F0 Model" `
     --natural-scale --base-blend 0 --pitch-offset-cents -50 `
     --out out/my-f0-v12.json
   ```

`--f0-teacher`には`id`と`cents`を持つJSONLを渡します。教師のF0はWORLDのHarvest（10ms、80〜800Hz）で求め、音量が最大から−40dB以下のフレームを除きます。前後150msの中央値から600cent以上離れた値をオクターブ単位で折り返し、σ20msで平滑化して、発話の中央値を0にします。読み込むときに値を100で割るため、自然スケール（log/0.3）に合わせて、`cents`には「セント×100/519.3」を記録します。モデルJSONへの変換では`--natural-scale`を指定します。

- `--plan-augment`: 学習の発話を合成時と同じ一定のモーラ長（120ms±15、休止180ms）に並べ直し、F0の目標をモーラごとに伸縮した複製を学習に加えます。同梱モデルの検証では、プランの時間でのF0の相関が0.59から0.75に上がりました。
- `--f0-dilations`: 受容野を広げて、文全体の抑揚の流れを学習させます。

---

## 2. 時間伸縮目標モデル（Speech Timing Target）

モーラの中で子音と母音を自然に伸縮させる比を決めるモデルです。言語ごとに学習した重み（`.safetensors`）をWORLD bridgeに埋め込んで使います。
