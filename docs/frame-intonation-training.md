# フレーム抑揚モデルの学習

## 準備

Python環境にはPyTorch、NumPy、pyopenjtalkが必要です。`--f0-source internal`では追加の実行ファイルは不要です。`--f0-source world`を使う場合は独自WORLDエンジンが必要です。ビルドスクリプトは、Windowsが`tools/build-world-engine.ps1`、Linuxが`tools/build-world-engine.sh`、macOSが`tools/build-world-engine-macos.sh`です。

学習データは`version: 1`のJSONLです。レコードは`id`、`audio_path`、`tokens`、`text`を持ちます。学習元コーパスの`--training-corpus`、`--training-corpus-license`、`--model-license`、`--license-notice`、`--source-notice`は必須です。コーパスの音声と台本を用意し、配布条件と出典を記録してください。

## 学習の流れ

1. コーパスの音声と台本を学習用JSONLへ変換します。
2. `train-frame-intonation-tcn.py`で学習します。
3. 候補モデルを聴取で比較し、採用を判断します。

```powershell
python tools/train-frame-intonation-tcn.py --dataset out/frame.jsonl `
  --f0-source internal --device cuda --holdout-test --epochs 24 --hidden 32 --batch-size 64 `
  --target-smooth-ms 80 --delta-weight 0.6 --f0-cache out/f0-cache `
  --model-id my-model-v1 --display-name "My intonation model" `
  --training-corpus "<学習元コーパス>" --training-corpus-license "<コーパスの配布条件>" `
  --model-license "MIT License" --license-notice licenses/MY-CORPUS.txt --source-notice licenses/MY-CORPUS.txt `
  --out out/my-model.json
```

## 学習オプション

`--holdout-test`はIDの固定ハッシュで学習・検証・テストを分離します。`--all-data-training`を指定すると監視用の文も学習に含めます。この場合の指標は学習データ内の性能を示します。未知文の候補比較には分離データを使います。`--target-smooth-ms`はF0教師の平滑化幅、`--delta-weight`は隣接フレーム差分損失の重みで、どちらも大きくすると抑揚が滑らかになります。`--f0-cache`は抽出したF0を記録し、再実行を高速化します。

各epochで生のピッチ誤差と再生時の処理を反映した誤差を記録します。後者は予測と教師の両方に平滑化・強度・振幅制限を適用した値です。この値が最小の重みを保存します。音声の自然さは別途聴取で評価します。

## 音素時刻のないコーパス

コーパスに音素時刻がない場合は`--alignment viterbi`を使います。これはOpen JTalkのアクセント注釈を弱い音響モデルとして、有声フレームがそのモーラの高低に近づくよう、モーラ長の上下限付きViterbiで境界を推定します。単純なDTWと違い、各モーラが妥当な長さに収まるため退化した経路になりません。

`prepare-kokoro-frame-data.py`では、`--world-engine`を付けるとWORLD HarvestでF0を推定し、省略すると高速な内蔵自己相関F0を使います。既定ではOpen JTalkの読みと一致するクリップだけを採用し、促音・長音の表記差で読みが異なるクリップも残す場合は`--allow-reading-mismatch`を指定します。`--alignment uniform`は均等配置、`--alignment viterbi`はアクセント注釈を使った整列です。

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
  --base-model models\frame-intonation-tcn-v9.1-t.json `
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
| `prepare-kokoro-frame-data.py` | `metadata.csv`と`wavs/<id>.wav`を学習用JSONLへまとめる（`id`・`text`・`reading`列） |
| `mora_alignment.py` | 音素時刻のないコーパス向けアクセントViterbiアラインメント |
| `train-frame-intonation-tcn.py` | フレーム抑揚モデルの学習と予測 |
| `train-manual-intonation-residual.py` | Intonation Labの手動調整から残差モデルを学習する |
| `frame_render_metrics.py` | 再生時のピッチ処理を反映した評価 |
| `cmd/tools/tts-eval` | 合成音声と原音選択の比較 |

これらはモデルの世代に依存しない共通ツールです。候補ごとの設定は実行引数と出力先で区別します。
