# モデルの学習

学習・データ準備コマンドは `cmd/tools/` にあります。Goの学習器はgograd v1.2.0を使用します。生成物とキャッシュは `out/` に置き、配布済みの `models/*.json` は上書きしません。モデルの配布条件は [モデル一覧](../models/README.md) を参照してください。

## 日本語フレーム抑揚モデル

`frame-intonation-tcn-v10` は、つくよみちゃんコーパス Vol.1 と「みんなで作る JSUT」basic5000 の音声を使い、Open JTalk のアクセント特徴と音素時刻から 10 ms ごとの F0 輪郭を学習します。入力は `version: 1` の JSONL で、各行に `id`、`text`、`audio_path`、時刻付き `tokens` を持ちます。Go トレーナーは gograd の残差 TCN を使用します。

仮名読みのコーパスは、Open JTalk 由来の音素列との厳密な照合ができない場合に `--allow-reading-mismatch` を指定します。音素列を含む入力では、休止記号や長母音などを Python 版と同じ規則で正規化して照合します。

```powershell
go run ./cmd/tools/prepare-minnade-jsut --root "data/みんなで作るJSUTコーパスbasic5000" --out out/jsut --start 1 --end 600
go run ./cmd/tools/prepare-intonation-frame-data --corpus out/jsut --allow-reading-mismatch --out out/frame.jsonl
```

初期のモーラ区間は発声区間へ均等に置きます。学習には MFA の `japanese_mfa` で整列し直した時刻を使います。MFA 辞書作成と整列結果の取り込みは Go コマンドが行います。MFA 本体は外部の Python/Kaldi バイナリです。

```powershell
go run ./cmd/tools/align-intonation-mfa run --out out/mfa out/frame.jsonl
```

`run` は外部の `mfa align` を起動し、`out/mfa/aligned.jsonl` を作ります。整列を分けて実行する場合は、次の `prepare` と `import` を使います。

```powershell
go run ./cmd/tools/align-intonation-mfa prepare --out out/mfa out/frame.jsonl
mfa align out/mfa/corpus out/mfa/dictionary.dict japanese_mfa out/mfa/alignments --output_format json --single_speaker --config_path out/mfa/config.yaml
go run ./cmd/tools/align-intonation-mfa import --alignments out/mfa/alignments --out out/frame-mfa.jsonl out/frame.jsonl
```

旧 Viterbi 整列はアクセントの高低だけを根拠に境界を選び、学習に不適切だったため削除しました。v10 の元データは `out/mfa-align-20261002/base-mfa.jsonl`（SHA-256 `12218686bb4ce92f51dca69b31df0ed850e78075fef3c2f015e8aba1ea7397a3`）です。ID の FNV-1a 分割で学習 458、検証 72、試験 70 文とし、内部 F0、10 ms フレーム、40 ms 教師平滑化、差分損失重み 0.35、AdamW 0.002、幅 32・dilation 1,2,4,8,16,32 の TCN で 24 epoch 学習しました。

```powershell
go run ./cmd/tools/train-frame-intonation `
  --dataset out/mfa-align-20261002/base-mfa.jsonl `
  --training-corpus "Tsukuyomi-chan Corpus Vol.1 + Minnade JSUT basic5000" `
  --license-notice licenses/TSUKUYOMI-CORPUS.txt `
  --license-notice licenses/MINNADE-JSUT-CORPUS.txt `
  --license-notice licenses/MFA-Japanese-NOTICE.txt `
  --device cpu --epochs 24 --hidden 32 --batch-size 32 `
  --dilations 1,2,4,8,16,32 --no-openjtalk-accent `
  --f0-cache out/mfa-align-20261002/f0-internal `
  --out out/frame-intonation-go/v10-candidate.json
```

`--holdout-test` は独立した試験文を確保します。`--all-data-training` は検証・試験文も学習に含めるため、得られる誤差はデータ内評価になります。`--target-smooth-ms` と `--delta-weight` は教師の平滑化と差分損失を調整します。`--index-only`、`--features-only` は事前確認、`--predict-corpus` と `--predict-out` は予測輪郭の書き出しに使います。CUDA 学習も選べます。

### Intonation Lab の手動残差

`.\build\qt\utautts.exe --intonation-lab` で編集したセッションを入力に、基準モデルへの手動補正を学習します。このモデルは配布されていません。

```powershell
go run ./cmd/tools/train-manual-intonation-residual --base-model models/frame-intonation-tcn-v10.json --out out/frame-intonation-lab.json --model-id frame-intonation-lab out/lab-session.utautts
```

## 英語フレーム抑揚モデル

`frame-intonation-tcn-en-v1` は LibriTTS-R の自然音声と MFA の ARPABET 音素時刻・語強勢を使います。英語前処理には Open JTalk を使いません。音素区間は MFA の境界を保ち、WORLD Harvest F0 をキャッシュします。WORLD DLL は Windows 専用です。他の OS では事前作成した F0 キャッシュを使います。

```powershell
go run ./cmd/tools/prepare-libritts-r-subset --archive data/libritts-r/downloads/train_clean_100.tar.gz --out out/libritts-subset --speakers 24 --per-speaker 40
mfa align out/libritts-subset english_us_arpa english_us_arpa out/english-frame/alignments --output_format json --num_jobs 4 --no_use_postgres
go run ./cmd/tools/prepare-english-frame-data --manifest out/libritts-subset/manifest.json --alignments out/english-frame/alignments --failed-list data/libritts-r/libritts_r_failed_speech_restoration_examples/train-clean-100_bad_sample_list.txt --out out/english-frame/corpus.jsonl
go run ./cmd/tools/cache-frame-f0 --dataset out/english-frame/corpus.jsonl --cache out/english-frame/f0-cache --world-engine runtime/utautts-world-engine.dll --workers 8
go run ./cmd/tools/train-frame-intonation --dataset out/english-frame/corpus.jsonl --language en --f0-source world --world-engine runtime/utautts-world-engine.dll --f0-cache out/english-frame/f0-cache --training-corpus "LibriTTS-R train_clean_100 subset" --model-license "CC BY 4.0" --license-notice licenses/LibriTTS-R-NOTICE.txt --license-notice licenses/MFA-English-ARPA-NOTICE.txt --device cpu --epochs 40 --hidden 32 --batch-size 16 --out out/english-frame/candidate.json
```

既定の分割は学習 854、検証 83、試験 101 文です。`corpus.audit.json` に除外理由と話者分割を記録します。

## 中国語声調抑揚モデル

`tone-intonation-zh-v1` は AISHELL-3 の音声、PaddleSpeech の音節時刻、WORLD Harvest の F0 を使い、声調規則輪郭に加える補正を学習します。20 特徴量から 5 点の補正量を ridge 回帰し、話者を分けて評価します。配布モデルの元学習には 3 話者から各 250 文を使いました。

Go の `train-mandarin-intonation` は、Parquet の音声とピンイン、PaddleSpeech の TextGrid、WORLD Harvest から観測を作り、回帰・評価・モデル JSON 出力まで行います。音声ごとの F0 は `out/` にキャッシュします。Parquet 読み込みに純 Go の `parquet-go` を使います。WORLD DLL による観測抽出は Windows 専用ですが、`--observations` で保存済み観測 JSONL を渡せば他の OS でも学習できます。`--collect-only --observations-out` は観測だけを書き出します。

```powershell
go run ./cmd/tools/train-mandarin-intonation --parquet data/aishell3/train-00000-of-00045.parquet --alignments data/aishell3/aishell3_alignment_tone --world-engine runtime/utautts-world-engine.dll --limit-per-speaker 250 --workers 4 --f0-cache out/tone-intonation-zh-v1/f0-cache --observations-out out/tone-intonation-zh-v1/observations.jsonl --out out/tone-intonation-zh-v1/candidate.json
```

### 既定の日本語抑揚 v11（Irodori-TTSを教師にしたF0ヘッド）

`intonation-ja-v11`のF0ヘッドは、Irodori-TTS v4.1-Small（MIT）にBASIC5000とUtauTTS用の日常文を読ませた音声から学習します。読みの照合（jsut-labelの正解の読みとOpenJTalkの読みが違う文を除く）とMFA整列のあと、自然F0を目標に学習します。

```powershell
go run ./cmd/tools/train-speech-timing --dataset out/irodori-teacher/train-ird.jsonl --alignments "out/mfa-align-20261002/alignments,out/irodori-teacher/alignments-ird" --cache out/irodori-teacher/features-ird.gob --language ja --f0 --valid 300 --steps 24000 --window 1000 --batch-size 8 --f0-dilations "1 2 4 8 16 32 64 1 2 4 8 16 32 64" --plan-augment --device cuda --out out/irodori-teacher/f0.safetensors
go run ./cmd/tools/package-f0-model --weights out/irodori-teacher/f0.safetensors --base models/frame-intonation-tcn-v10.json --id my-f0-v1 --display-name "My F0 v1" --base-blend 0.65 --use-energy=false --out out/my-f0-v1.json
```

- `--f0-dilations`はF0ブランチの受容野。既定（1〜8×2、約±0.6秒）では文全体の抑揚を学べないため、1〜64×2（約±5秒）を使います。
- `--plan-augment`は学習発話を合成時と同じ一定のモーラ長（120ms±15、休止180ms）へ並べ直し、F0目標をモーラごとに伸縮した複製を学習へ足します。合成時はプラン時間で推論するため、これが無いと自然時間との差で精度が大きく落ちます（検証のプラン時間の相関 0.59→0.75）。
- 検証では、発話ごとのF0の相関を自然時間（`f0r`）とプラン時間（`f0r_plan`）で表示します。試聴前の比較に使います。
- `--f0-position`（文内の位置の特徴）は効果が無かったため既定では使いません。
- v11は基準モデルv10の曲線を0.65混ぜ（`--base-blend`、JSONの`base_blend`）、エネルギーヘッドを使いません（`--use-energy=false`）。

## 日本語の目標音素時間モデル

`speech-timing-target-v1` は音素・長さ・相対 F0 から、80 帯域の正規化対数メル包絡を予測します。Go の gograd コマンドが MFA 音素時刻と WORLD フレームを読み、特徴をキャッシュして学習します。特徴抽出には Windows の WORLD DLL が必要ですが、作成済みキャッシュからの学習は他の OS でも可能です。

```powershell
go run ./cmd/tools/train-speech-timing --dataset out/mfa-align-20261002/base-mfa.jsonl --alignments out/mfa-align-20261002/alignments --world-engine runtime/utautts-world-engine.dll
```

`--features-only --features-json out/speech-timing-target/sample.json` は先頭の特徴を比較用に出力します。既存の `--cache` があれば、`--dataset`、`--alignments`、WORLD DLLなしで学習できます。

### 日本語の統合韻律モデル（F0・エネルギー）

`speech-timing-target-ja-prosody-v1`（`models/speech-timing-target-ja-prosody-v1.json`）は、メルトランクに加えて F0 とエネルギーのヘッドを持ちます。抑揚モデルとしてIDで選ぶと、日本語の抑揚をこのモデルで生成します（試用、`default_priority` 90）。既定は従来の抑揚モデル（`frame-intonation-tcn-v10`）です。

F0教師は既存の抑揚モデルの**ランタイム輪郭**（平滑化・p99・最大90centクリップ込み）をコーパスの時間軸で生成した蒸留教師です。

```powershell
# 1. 教師輪郭（自然時間軸）
go run ./cmd/tools/prosody-teacher --dataset out/mfa-align-20261002/all-mfa.jsonl --model models/frame-intonation-tcn-v10.json --out out/speech-timing-target/ja-v10-teacher.jsonl

# 2. プラン時間風（モーラ120ms±15・ポーズ180ms）のデータと教師を作り、自然時間のデータと混合する
#    plan-mfa.jsonl（plan_timing: true、時刻を振り直したトークン）を用意し、同じツールで教師を生成。
#    all-mfa.jsonl + plan-mfa.jsonl = mix-mfa.jsonl、教師も連結して ja-mix-teacher.jsonl にする。

# 3. 学習（F0蒸留＋エネルギー、プラン側はメル・エネルギー目標をNaNで除外）
go run ./cmd/tools/train-speech-timing --dataset out/mfa-align-20261002/mix-mfa.jsonl --alignments "out/mfa-align-20261002/alignments,out/mfa-align-20261002/alignments-others" --cache out/speech-timing-target/ja-mix-features.gob --language ja --f0 --f0-teacher out/speech-timing-target/ja-mix-teacher.jsonl --f0-weight 1 --f0-delta-weight 0.35 --energy-weight 1 --lr 0.0015 --steps 8000 --device cuda --out out/speech-timing-target/ja-mh.safetensors
```

学習した重みは`package-f0-model`でモデルJSONにまとめ、基準モデルと同じ`models/`へ置きます。モデルJSONは`base_model`（同じディレクトリの基準の抑揚モデル）と`f0_head`（safetensorsのbase64）を持ち、基準モデルがアクセント特徴とモーラの予測を、F0ヘッドが自動ピッチ曲線とモーラの音量を担います。ライセンス・通知・出典は基準モデルから引き継ぎます。

```powershell
go run ./cmd/tools/package-f0-model --weights out/speech-timing-target/ja-mh.safetensors --base models/frame-intonation-tcn-v10.json --id my-f0-v1 --display-name "My F0 v1" --priority 90 --out out/my-f0-v1.json
```

F0ヘッドは教師のcent/100スケール（metadata `f0_scale=100`）で出力し、実行時に発話区間のGaussian平滑化（20ms）とp99/最大クリップを適用します。エネルギーは発話内で中心化し、プランの`EnergyFactor`（0.75〜1.3）へ適用します。言語が日本語以外のときは従来の経路（言語別抑揚モデル）へ戻ります。

### 英語の目標音素時間モデル

`speech-timing-target-en-v1` は LibriTTS-R のトークン境界（`out/english-frame-v1/corpus.jsonl`）から学習します。`--corpus` は音素区間付きのトークン列を読み、言語別の語彙を組み立ててモデルmetadataの`phones`へ保存します。

```powershell
go run ./cmd/tools/train-speech-timing `
  --corpus out/english-frame-v1/corpus.jsonl --language en `
  --cache out/speech-timing-target/en-features.gob --features-only

go run ./cmd/tools/train-speech-timing `
  --corpus out/english-frame-v1/corpus.jsonl --language en `
  --cache out/speech-timing-target/en-features.gob `
  --steps 6000 --valid 30 --seed 0 --device cuda `
  --training-corpus "LibriTTS-R train_clean_100 subset (24 speakers, MFA english_us_arpa)" `
  --license-notice licenses/LibriTTS-R-NOTICE.txt `
  --license-notice licenses/MFA-English-ARPA-NOTICE.txt `
  --out out/speech-timing-target/en-model-v1.safetensors `
  --fixture out/speech-timing-target/en-parity-v1.json
```

学習した重みは `internal/speechtiming/speech-timing-target-en-v1.safetensors` へ置き、`TargetForLanguage("en")` が選びます。英語の時間伸縮はプランの `phone_timings`（codaを含む音素区間）をモデルへ渡します。

### 中国語の目標音素時間モデル

`speech-timing-target-zh-v1` は AISHELL-3（PaddleSpeechのtone TextGrid）から学習します。`prepare-aishell3-timing` がparquetの音声をWAVへ展開し、単語tierの音節区間と音素tierのinitial/final境界から、runtime記号（声調を除くpinyin音素）の区間を持つcorpusを作ります。

```powershell
go run ./cmd/tools/prepare-aishell3-timing `
  --parquet data/aishell3/train-00000-of-00045.parquet `
  --alignments data/aishell3/aishell3_alignment_tone `
  --out-audio out/aishell3-zh-timing/audio `
  --out-corpus out/aishell3-zh-timing/corpus.jsonl

go run ./cmd/tools/train-speech-timing `
  --corpus out/aishell3-zh-timing/corpus.jsonl --language zh `
  --cache out/speech-timing-target/zh-features.gob --features-only

go run ./cmd/tools/train-speech-timing `
  --corpus out/aishell3-zh-timing/corpus.jsonl --language zh `
  --cache out/speech-timing-target/zh-features.gob `
  --steps 6000 --valid 30 --seed 0 --device cuda `
  --training-corpus "AISHELL-3 (PaddleSpeech tone alignment)" `
  --license-notice licenses/AISHELL-3-NOTICE.txt `
  --license-notice licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt `
  --out out/speech-timing-target/zh-model-v1.safetensors `
  --fixture out/speech-timing-target/zh-parity-v1.json
```

学習した重みは `internal/speechtiming/speech-timing-target-zh-v1.safetensors` へ置き、`TargetForLanguage("zh")` が選びます。

### 中断と再開

`--out` は最良の検証結果の推論用重み、`--fixture` はその重みに対応するparity fixtureです。再開用の学習状態は `--checkpoint` へ別に保存します。省略時は `<out>.training.safetensors` です。重み・AdamW・OneCycle・分割・窓サンプラの乱数状態・最良モデルを含み、推論用重みだけからは再開できません。

```powershell
go run ./cmd/tools/train-speech-timing `
  --cache out/speech-timing-target/go-features.gob `
  --steps 6000 --valid 30 --seed 0 --device cpu `
  --out out/speech-timing-target/first.safetensors `
  --fixture out/speech-timing-target/first-parity.json `
  --checkpoint out/speech-timing-target/training.safetensors `
  --checkpoint-every 250 --stop-after 2000

go run ./cmd/tools/train-speech-timing `
  --cache out/speech-timing-target/go-features.gob `
  --steps 6000 --valid 30 --seed 0 --device cpu `
  --out out/speech-timing-target/resumed.safetensors `
  --fixture out/speech-timing-target/resumed-parity.json `
  --checkpoint out/speech-timing-target/training.safetensors `
  --resume out/speech-timing-target/training.safetensors
```

`--steps` は追加step数ではなく、最初に予定した全step数です。再開時もキャッシュの内容、seed、検証数、batch/windowサイズ、評価間隔、コーパス・通知指定を同じにします。`--out` と `--fixture` は新しいパスを指定してください。最良モデルは学習状態に保持されるため、再開後にスコアが改善しなくても書き出せます。

`--batch-size` は既定16、`--window` は400 frame、`--eval-every` と `--checkpoint-every` は250 stepです。`--stop-after` は停止する完了step数で、0なら最後まで実行します。Ctrl+Cでは実行中の更新を終えて保存し、強制終了では最後の定期保存から再開します。parity fixture作成より先に、最新の学習状態を保存します。

CPUの再開は連続実行とファイル単位で一致します。CUDAは勾配集約順序によるfloat32の微小差を許して検証します。窓サンプラは復元可能なPCG乱数を使い、最後の完全な窓も抽選対象にするため、旧トレーナーとは同じseedでも新規学習の軌跡が異なります。

## 残る Python

MFA 本体は外部の Python/Kaldi 依存です。原音区間の整列・探索・監査、MFA の前後処理、音響プロット、wasm 配布補助は `cmd/tools/` の Go コマンドです。Open JTalk 実行時ブリッジとそのビルド検証・PyInstaller ライセンス収集は、ネイティブの辞書駆動 NJD 生成が Go から利用できるまで Python に残します。必要機能と検証結果は [Open JTalk 実行時ブリッジの移行判定](openjtalk-bridge-audit.md) を参照してください。

性能測定は `go run ./cmd/tools/performance-baseline --out out/perf-... --voicebank <音源>` で行います。ベンチ、ビルド、合成結果、pprofを新しいディレクトリへまとめます。
