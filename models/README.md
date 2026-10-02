# 抑揚モデルのライセンス

この文書は同梱モデルの出典と通知の入口です。条件の原文は各配布元の規約に従います。全体の入口は[ライセンスの適用範囲](../LICENSE-SCOPE.md)です。

## 同梱モデル

日本語の既定モデル: `frame-intonation-tcn-v10`。試用: `frame-intonation-tcn-v10-mora-duration-v1`（モーラ長も予測）。代替: `frame-intonation-tcn-v9.1-t`、`frame-intonation-tcn-v9-t`。英語の既定モデル: `frame-intonation-tcn-en-v1`。中国語の既定モデル: `tone-intonation-zh-v1`。日本語・英語の学習と評価: [フレーム抑揚モデルの学習](../docs/frame-intonation-training.md)

`frame-intonation-tcn-v10`と`frame-intonation-tcn-v9-*`は10ms単位の相対ピッチだけを予測します。モーラ長はGUIで指定した基準値と、言語別の時間規則（「ん」0.9倍、「ー」1.2倍など）で決めます。

## v10 Tsukuyomi + JSUT（既定）

`frame-intonation-tcn-v10`は、v9.1と同じ[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)（CV.夢前黎）と[みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/)のBASIC5000_0001-0600を、Montreal Forced Aligner（`japanese_mfa`）で合成時と同じモーラ区間に整列して学習したモデルです。v9.1のアクセントViterbi整列ではモーラ境界が大きくずれており、合成時の抑揚が約90ms早くなっていました。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本とMFA整列モデルを再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)、[licenses/MFA-Japanese-NOTICE.txt](../licenses/MFA-Japanese-NOTICE.txt)

## v10 + モーラ長 v1（試用）

`frame-intonation-tcn-v10-mora-duration-v1`は、v10と同じ抑揚に、モーラ長を予測するheadを加えたモデルです。モーラ長は、v10と同じコーパス（夢前黎さんの読み上げ）をMontreal Forced Alignerで整列した、合成時と同じモーラ区間（母音の始まり〜次の母音の始まり）の長さを、発話ごとの中央値に対する倍率として学習しました。休止の長さは予測せず、GUIの設定のままです。

- 倍率は0.5〜2.0倍に制限し、句末（休止・文末の直前）のモーラは1.0倍以上、文頭・休止の直後のモーラは1.25倍以下にします。自然音声の句末の母音は短く、そのまま使うと語尾が欠けるためです
- 同じ文の自然音声のモーラ長を使うと現行より自然になりましたが、予測では文や音源によって間延び・詰まりが出ることがあります。既定はv10のままです
- 学習: `tools/train-mora-duration-tcn.py`。重みはMIT Licenseで配布します
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)、[licenses/MFA-Japanese-NOTICE.txt](../licenses/MFA-Japanese-NOTICE.txt)

## v9.1 Tsukuyomi + JSUT（代替）

`frame-intonation-tcn-v9.1-t`は、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)（CV.夢前黎）と[みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/)のBASIC5000_0001-0600で学習したモデルです。みんなで作るJSUTは複数話者の寄せ集めです。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本を再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)

## v9 Tsukuyomi（代替）

`frame-intonation-tcn-v9-t`は、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)で学習した単一話者モデルです。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本を再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)

## English Frame Intonation TCN v1（英語の既定）

`frame-intonation-tcn-en-v1`は、[LibriTTS-R](https://www.openslr.org/141/)のtrain-clean-100から選んだ発話で学習した英語モデルです。

- 本モデルは10ms単位の相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはCC BY 4.0で配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本とMFA整列モデルを再配布しません
- 重みの通知: [licenses/ENGLISH-FRAME-INTONATION-TCN-V1.txt](../licenses/ENGLISH-FRAME-INTONATION-TCN-V1.txt)、ライセンス本文: [licenses/CC-BY-4.0.txt](../licenses/CC-BY-4.0.txt)
- 学習元の通知: [licenses/LibriTTS-R-NOTICE.txt](../licenses/LibriTTS-R-NOTICE.txt)、[licenses/MFA-English-ARPA-NOTICE.txt](../licenses/MFA-English-ARPA-NOTICE.txt)

Delta/VCCVの音節構成を対象とし、en-cv/en-arpasingへの適用は未評価です。学習条件と評価はモデルJSONの`training`と`metrics`を参照してください。旧`english-intonation-v1`の選択設定はこのモデルへ移行します。

## Mandarin Tone Intonation v1（中国語の既定）

`tone-intonation-zh-v1`は、[AISHELL-3](https://www.openslr.org/93/)の2話者の発話で学習し、別の1話者で検証した中国語モデルです。声調規則による音高曲線へ、有界な補正だけを加えます。

- 本モデルは相対ピッチ（抑揚）の補正のみを学習し、話者の声質を意図的に再現しません
- 重みはApache License 2.0で配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本と整列データを再配布しません
- 重みの通知: [licenses/TONE-INTONATION-ZH-V1.txt](../licenses/TONE-INTONATION-ZH-V1.txt)、ライセンス本文: [licenses/APACHE-2.0.txt](../licenses/APACHE-2.0.txt)
- 学習元の通知: [licenses/AISHELL-3-NOTICE.txt](../licenses/AISHELL-3-NOTICE.txt)、[licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt](../licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt)

`zh-cvvc`を対象とします。学習条件と評価はモデルJSONの`training`・`metrics`に記録しています。

## モデルの記録

モデルJSONの`id`、`display_name`、`license`、`license_notices`、`provenance`、`training`、`metrics`に、学習元コーパス、ライセンス、学習条件、評価指標を記録します。`license_notices`は使用した各データの通知を列挙します。配布物にはモデルJSONと通知を含めます。
