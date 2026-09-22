# Intonation Lab

Intonation Lab は、通常の UtauTTS の編集画面を使って手動調整の教師データを作るためのモードです。基本編集と拡張編集をそのまま使い、専用画面は起動しません。

```powershell
.\build\qt\utautts.exe --intonation-lab
```

起動すると、50 文の例文が順に表示されます。上段には現在の文だけが表示され、右側の音源・速度などの設定欄や発話追加は隠れます。自動予測と合成には、`models/frame-intonation-v9-t.json` を基準モデルとして使用します。

## 操作

1. 基本編集または拡張編集で、音の高さ・タイミング・発音設定を調整します。
2. 必要に応じて再生して確認します。
3. 右上の「完了して次へ」を押します。

完了時には、調整結果が Documents フォルダーの `intonation-lab-<日時>.utautts` に自動保存され、次の文の解析と抑揚予測が始まります。完了済みの文だけが `training_accepted: true` として保存されるため、途中で終えても保存済みの文だけを学習に使えます。

保存済みのセッションを再開したい場合は、Lab モードの「ファイル」→「開く」から対象の `.utautts` を開いてください。

## 残差モデルの学習

少なくとも 8 文を完了した後、保存されたセッションを指定して学習します。

```powershell
python tools\train-manual-intonation-residual.py <lab-session.utautts> `
  --base-model models\frame-intonation-v9-t.json `
  --out out\frame-intonation-v9-lab.json `
  --model-id frame-intonation-v9-lab `
  --display-name "Frame intonation TCN v9 Lab"
```

複数のセッションファイルを並べて指定することもできます。学習結果は、V9F の自動抑揚へ手動調整の傾向を加える残差モデルです。
