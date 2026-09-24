# 抑揚モデルのライセンス

## 同梱モデル

日本語の既定モデル: `frame-intonation-v9-t`。代替: `frame-intonation-v9-k`。英語モデル: `english-intonation-v1`。学習と評価: [フレーム抑揚モデルの学習](../docs/frame-intonation-training.md)

`frame-intonation-v9-*`は10ms単位の相対ピッチだけを予測します。モーラ長はGUIで指定した基準値と、言語別の時間規則（「ん」0.9倍、「ー」1.2倍など）で決めます。

## v9 Tsukuyomi（既定）

`frame-intonation-v9-t`は、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)で学習した単一話者モデルです。

- 音声: 商用・非商用可。クレジット必須。音声そのものの再配布は不可
- 台本: 声優統計／JVSコーパスの音素バランス文（CC BY-SA 4.0）。音声の配布は著作権法第30条の4によるためコピーレフトは継承しません
- 条件: [つくよみちゃんコーパスの通知](../licenses/TSUKUYOMI-CORPUS.txt)

## v9 Kokoro（代替）

`frame-intonation-v9-k`は、[Kokoro Speech Dataset](https://github.com/kaiidams/Kokoro-Speech-Dataset)で学習した単一話者モデルです。

- 音声: パブリックドメイン（米国、他国も概ねPD）
- 出典: [Kokoro Speech Datasetの通知](../licenses/KOKORO-SPEECH-DATASET.txt)

## English

`english-intonation-v1`はUtauTTS用の係数モデルで、ARPAbetの強勢、語境界、句境界から相対ピッチを予測します。英語のカードでは自動で選ばれます。配布条件: [ライセンス](../licenses/ENGLISH-INTONATION-V1.txt)のMIT License。

## モデルの記録

モデルJSONの`id`、`display_name`、`license`、`license_notice`、`provenance`、`training`、`metrics`に、学習元コーパス、ライセンス、学習条件、評価指標を記録します。コーパスやモデルごとに条件を記録し、配布物にはモデルJSONと通知を含めます。
