# 抑揚モデルのライセンス

## 同梱モデル

日本語の既定モデル: `frame-intonation-v8`。ピッチとモーラ長を予測するモデル: `prosody-multitask-v1`。学習と評価: [フレーム抑揚モデルの学習](../docs/frame-intonation-training.md)

英語モデル: `english-intonation-v1`。ARPAbetの強勢、語境界、句境界からピッチと長さを予測します。英語のカードでは、このモデルを優先します。

## JSUT由来モデル

JSUT音声を使った同梱モデル: `frame-intonation-v8`、`prosody-multitask-v1`。[JSUT公式ページの利用条件](https://sites.google.com/site/shinnosuketakamichi/publication/jsut)を基準にしたUtauTTSの配布条件は、学術研究、非商用研究、個人利用です。商用利用にはJSUT権利者の事前許諾が必要です。

モデルの配布条件: UtauTTSのプロジェクト方針。出典データの条件: [JSUTとラベルの通知](../licenses/JSUT-DATA-AND-LABELS.txt)。配布物: モデルJSONとライセンス通知。

学習ツールの入力: jsut-labelのBASIC5000ラベルによる音素区間と時刻。`frame-intonation-v8`のアクセントと品詞特徴: Open JTalk。`--jsut-context-labels`使用時のアクセント特徴: jsut-label。モデルの配布条件: [抑揚モデルの通知](../licenses/PROSODY-MODELS.txt)。

モデルJSONの`license`は配布条件、`license_notice`は通知ファイルのパスです。コーパスやモデルごとに条件を記録します。

## 英語モデル

`english-intonation-v1`はUtauTTS用の係数モデルで、`licenses/ENGLISH-INTONATION-V1.txt`のMIT Licenseで配布します。
