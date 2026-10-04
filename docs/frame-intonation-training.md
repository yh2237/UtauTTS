# フレーム抑揚モデルの学習

この文書は日本語モデルの学習手順です。英語モデルは[英語フレーム抑揚モデルの学習](english-frame-intonation-training.md)を参照してください。

## 準備

学習はgogradを使うGoのコマンド`cmd/tools/train-frame-intonation`で行えます（[手順](go-frame-intonation-training.md)）。v10・en-v1と同じ特徴量・損失で同等の検証誤差になり、CUDAでも学習できます。以下のPython版（`train-frame-intonation-tcn.py`）は、データ準備の補助スクリプトが一部を共有しているため当面残します。

Python版を使う場合、Python環境にはPyTorch、NumPy、pyopenjtalkが必要です。`--f0-source internal`では追加の実行ファイルは不要です。`--f0-source world`を使う場合は独自WORLDエンジンが必要です。ビルドスクリプトは、Windowsが`tools/build-world-engine.ps1`、Linuxが`tools/build-world-engine.sh`、macOSが`tools/build-world-engine-macos.sh`です。

学習データは`version: 1`のJSONLです。レコードは`id`、`audio_path`、`tokens`、`text`を持ちます。`--training-corpus`、`--model-license`、`--license-notice`（複数指定可）は必須です。コーパスの音声と台本を用意し、出典と、使用した各データの通知を記録してください。

## 学習の流れ

1. コーパスの音声と台本を学習用JSONLへ変換します。
2. `train-frame-intonation-tcn.py`で学習します。
3. 候補モデルを聴取で比較し、採用を判断します。

```powershell
python tools/train-frame-intonation-tcn.py --dataset out/frame.jsonl `
  --f0-source internal --device cuda --holdout-test --epochs 24 --hidden 32 --batch-size 64 `
  --target-smooth-ms 80 --delta-weight 0.6 --f0-cache out/f0-cache `
  --model-id my-model-v1 --display-name "My intonation model" `
  --training-corpus "<学習元コーパス>" `
  --model-license "MIT License" --license-notice licenses/MY-CORPUS.txt `
  --out out/my-model.json
```

## 学習オプション

`--holdout-test`はIDの固定ハッシュで学習・検証・テストを分離します。`--all-data-training`を指定すると監視用の文も学習に含めます。この場合の指標は学習データ内の性能を示します。未知文の候補比較には分離データを使います。`--target-smooth-ms`はF0教師の平滑化幅、`--delta-weight`は隣接フレーム差分損失の重みで、どちらも大きくすると抑揚が滑らかになります。`--f0-cache`は抽出したF0を記録し、再実行を高速化します。

各epochで生のピッチ誤差と再生時の処理を反映した誤差を記録します。後者は予測と教師の両方に平滑化・強度・振幅制限を適用した値です。この値が最小の重みを保存します。音声の自然さは別途聴取で評価します。

## 音素時刻のないコーパス

コーパスに音素時刻がない場合は、Montreal Forced Aligner（MFA）の日本語音響モデル（`japanese_mfa`）で整列します。`prepare-intonation-frame-data.py`で作ったJSONLを、`align-intonation-mfa.py`で整列し直します。

```powershell
python tools/align-intonation-mfa.py prepare --out out/mfa out/frame.jsonl
mfa align out/mfa/corpus out/mfa/dictionary.dict japanese_mfa out/mfa/alignments `
  --output_format json --single_speaker --config_path out/mfa/config.yaml
python tools/align-intonation-mfa.py import --alignments out/mfa/alignments --out out/frame-mfa.jsonl out/frame.jsonl
```

`prepare`は各モーラを1語とし、モーラごとの音素列を明示した辞書を作ります。`import`はモーラ区間を合成時と同じ定義（UTAUのノート: 母音の始まりから次のモーラの母音の始まりまで）にします。学習と合成でモーラ区間の定義がずれると、予測する抑揚が時間的にずれます。

`prepare-intonation-frame-data.py`の`--alignment viterbi`は、Open JTalkのアクセント注釈を弱い音響モデルとして境界を推定します。アクセントの高低だけでは境界が決まりにくく、多くのモーラが長さの上下限（60msと300ms）に張り付くため、学習には使いません。MFAで整列し直すと、テスト文の自然音声F0との相関は0.34から0.67に上がりました（合成時と同じモーラ区間で評価）。`--alignment uniform`は均等配置です。

`prepare-intonation-frame-data.py`では、`--world-engine`を付けるとWORLD HarvestでF0を推定し、省略すると高速な内蔵自己相関F0を使います。既定では`metadata.csv`の読み列（音素列）がOpen JTalkの音素と一致するクリップだけを採用します。読み列が仮名の場合（`prepare-minnade-jsut.py`の出力など）は照合できず全件が除外されるため、`--allow-reading-mismatch`を指定します。

学習（`train-frame-intonation-tcn.py`）では、`--f0-source world`がWORLD HarvestでF0教師を作り、`--f0-cache`が抽出したF0を記録して再実行を高速化します。

## Intonation Labで教師データを作る

Intonation Labは、通常のUtauTTSの編集画面を使って手動調整の教師データを作るためのモードです。基本編集と拡張編集をそのまま使い、専用画面は起動しません。

```powershell
.\build\qt\utautts.exe --intonation-lab
```

起動すると例文が順に表示されます。上段には現在の文だけが表示され、右側の音源・速度などの設定欄や発話追加は隠れます。自動予測と合成には基準モデルを使用します。

1. 基本編集または拡張編集で、音の高さ・タイミング・発音設定を調整します。
2. 必要に応じて再生して確認します。
3. 右上の「完了して次へ」を押します。

完了時には調整結果がDocumentsフォルダーの`intonation-lab-<日時>.utautts`に自動保存され、次の文の解析と抑揚予測が始まります。完了済みの文だけが`training_accepted: true`として保存されるため、途中で終えても保存済みの文だけを学習に使えます。保存済みのセッションを再開したい場合は、Labモードの「ファイル」→「開く」から対象の`.utautts`を開いてください。

少なくとも8文を完了した後、保存されたセッションを指定して残差モデルを学習します。

```powershell
python tools\train-manual-intonation-residual.py <lab-session.utautts> `
  --base-model models\frame-intonation-tcn-v10.json `
  --out out\frame-intonation-tcn-v9-lab.json `
  --model-id frame-intonation-tcn-v9-lab `
  --display-name "Frame intonation TCN v9 Lab"
```

複数のセッションファイルを並べて指定することもできます。学習結果は、基準モデルの自動抑揚へ手動調整の傾向を加える残差モデルです。

## 聴取比較

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/japanese-bank" --renderers utautts-world-phrase --model-file out/frame-intonation-candidate.json --repeat 1 --out out/candidate-listening
```

比較条件は既定モデルと同じ文章・音源・Renderer・設定にします。合成計画の原音選択と時間配置を確認し、学習にない文章の試聴で採用を判断します。評価項目は[読み上げ品質の評価](../tools/evaluation/README.md)を参照してください。

## ツールの役割

| ツール | 用途 |
| --- | --- |
| `prepare-intonation-frame-data.py` | `metadata.csv`（`id`・`text`・`reading`列）と`wavs/<id>.wav`を学習用JSONL（`id`・`text`・`source_reading`・`openjtalk_reading`など）へまとめる |
| `align-intonation-mfa.py` | 学習用JSONLのモーラ時刻をMFAの強制整列で作り直す |
| `mora_alignment.py` | アクセントViterbiアラインメント（境界が上下限に張り付くため学習には使わない） |
| `train-frame-intonation-tcn.py` | フレーム抑揚モデルの学習と予測 |
| `train-manual-intonation-residual.py` | Intonation Labの手動調整から残差モデルを学習する |
| `frame_render_metrics.py` | 再生時のピッチ処理を反映した評価 |
| `cmd/tools/tts-eval` | 合成音声と原音選択の比較 |

これらはモデルの世代に依存しない共通ツールです。候補ごとの設定は実行引数と出力先で区別します。
