# 抑揚モデルのライセンス

この文書は同梱モデルの出典と通知の入口です。条件の原文は各配布元の規約に従います。全体の入口は[ライセンスの適用範囲](../LICENSE-SCOPE.md)です。

## 同梱モデル

日本語の既定モデル: `frame-intonation-tcn-v9.1-t`。代替: `frame-intonation-tcn-v9-t`。英語モデル: `english-intonation-v1`。学習と評価: [フレーム抑揚モデルの学習](../docs/frame-intonation-training.md)

`frame-intonation-tcn-v9-*`は10ms単位の相対ピッチだけを予測します。モーラ長はGUIで指定した基準値と、言語別の時間規則（「ん」0.9倍、「ー」1.2倍など）で決めます。

## v9.1 Tsukuyomi + JSUT（既定）

`frame-intonation-tcn-v9.1-t`は、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)（CV.夢前黎）と[みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/)のBASIC5000_0001-0600で学習したモデルです。みんなで作るJSUTは複数話者の寄せ集めです。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本を再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)

## v9 Tsukuyomi（代替）

`frame-intonation-tcn-v9-t`は、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)で学習した単一話者モデルです。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本を再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)

## English

`english-intonation-v1`はUtauTTS用の係数モデルで、ARPAbetの強勢、語境界、句境界から相対ピッチを予測します。英語のカードでは自動で選ばれます。配布条件: [ライセンス](../licenses/ENGLISH-INTONATION-V1.txt)のMIT License。

## モデルの記録

モデルJSONの`id`、`display_name`、`license`、`license_notices`、`provenance`、`training`、`metrics`に、学習元コーパス、ライセンス、学習条件、評価指標を記録します。`license_notices`は使用した各データの通知を列挙します。配布物にはモデルJSONと通知を含めます。
