# 抑揚モデルのライセンス

この文書は同梱モデルの条件の一次情報です。全体の入口は[ライセンスの適用範囲](../LICENSE-SCOPE.md)です。

## 同梱モデル

日本語の既定モデル: `frame-intonation-tcn-v9.1-t`。代替: `frame-intonation-tcn-v9-t`、`frame-intonation-tcn-v9-k`。英語モデル: `english-intonation-v1`。学習と評価: [フレーム抑揚モデルの学習](../docs/frame-intonation-training.md)

`frame-intonation-tcn-v9-*`は10ms単位の相対ピッチだけを予測します。モーラ長はGUIで指定した基準値と、言語別の時間規則（「ん」0.9倍、「ー」1.2倍など）で決めます。

## v9.1 Tsukuyomi + JSUT（既定）

`frame-intonation-tcn-v9.1-t`は、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)と[みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/)の夢前黎さん担当分（BASIC5000_0001-0600）で学習した単一話者モデルです。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用規約は、配布元が公開する原文に従います
- 通知: [licenses/FRAME-INTONATION-V9.1-T.txt](../licenses/FRAME-INTONATION-V9.1-T.txt)

## v9 Tsukuyomi（代替）

`frame-intonation-tcn-v9-t`は、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)で学習した単一話者モデルです。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用規約は、配布元が公開する原文に従います
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)

## v9 Kokoro（代替）

`frame-intonation-tcn-v9-k`は、[Kokoro Speech Dataset](https://github.com/kaiidams/Kokoro-Speech-Dataset)で学習した単一話者モデルです。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を再現しません
- 重みはMIT Licenseで配布します
- 通知: [licenses/KOKORO-SPEECH-DATASET.txt](../licenses/KOKORO-SPEECH-DATASET.txt)

## English

`english-intonation-v1`はUtauTTS用の係数モデルで、ARPAbetの強勢、語境界、句境界から相対ピッチを予測します。英語のカードでは自動で選ばれます。配布条件: [ライセンス](../licenses/ENGLISH-INTONATION-V1.txt)のMIT License。

## モデルの記録

モデルJSONの`id`、`display_name`、`license`、`license_notice`、`provenance`、`training`、`metrics`に、学習元コーパス、ライセンス、学習条件、評価指標を記録します。コーパスやモデルごとに条件を記録し、配布物にはモデルJSONと通知を含めます。
