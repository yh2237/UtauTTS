# 抑揚モデルのライセンス

この文書は同梱モデルの出典と通知の入口です。条件の原文は各配布元の規約に従います。全体の入口は[ライセンスの適用範囲](../LICENSE-SCOPE.md)です。

## 同梱モデル

日本語の既定モデル: `frame-intonation-tcn-v11`。代替: `frame-intonation-tcn-v10`、`frame-intonation-tcn-v9.1-t`。英語の既定モデル: `frame-intonation-tcn-en-v1`。中国語の既定モデル: `tone-intonation-zh-v1`。学習と評価: [モデルの学習](../docs/model-training.md)

`frame-intonation-tcn-v10`と`frame-intonation-tcn-v9.1-t`は10ms単位の相対ピッチだけを予測します。モーラ長はGUIで指定した基準値と、言語別の時間規則（「ん」0.9倍、「ー」1.2倍など）で決めます。

## Frame Intonation TCN v11（既定）

`frame-intonation-tcn-v11`は、`frame-intonation-tcn-v10`の抑揚曲線（重み0.65）と、[Irodori-TTS](https://huggingface.co/Aratako/Irodori-TTS-v4.1-Small)（MIT）にBASIC5000とUtauTTS用の日常文を読ませた音声で学習したF0ヘッドの曲線（0.35）を混ぜます。アクセント特徴とモーラの予測はv10が担います。F0ヘッドは合成時と同じ一定のモーラ長に並べ直した複製も使い、文全体（約±5秒）を見て学習しています。同じモデルのエネルギーヘッドでモーラの音量（0.75〜1.3倍）も変えます。聴取では混合が15行中7行でv10より選ばれ（v10 4、同程度4）、音量ありは音量なしに対し10対5で選ばれました。

- 本モデルは相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはMIT Licenseで配布します。教師の音声・台本・音声合成モデルは再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)、[licenses/MFA-Japanese-NOTICE.txt](../licenses/MFA-Japanese-NOTICE.txt)、[licenses/IRODORI-TEACHER-NOTICE.txt](../licenses/IRODORI-TEACHER-NOTICE.txt)

## v10 Tsukuyomi + JSUT（代替）

`frame-intonation-tcn-v10`は、v9.1と同じ[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)（CV.夢前黎）と[みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/)のBASIC5000_0001-0600を、Montreal Forced Aligner（`japanese_mfa`）で合成時と同じモーラ区間に整列して学習したモデルです。v9.1のアクセントViterbi整列ではモーラ境界が大きくずれており、合成時の抑揚が約90ms早くなっていました。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本とMFA整列モデルを再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)、[licenses/MFA-Japanese-NOTICE.txt](../licenses/MFA-Japanese-NOTICE.txt)

## v9.1 Tsukuyomi + JSUT（代替）

`frame-intonation-tcn-v9.1-t`は、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)（CV.夢前黎）と[みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/)のBASIC5000_0001-0600で学習したモデルです。みんなで作るJSUTは複数話者の寄せ集めです。

- 本モデルはフレーム単位の相対ピッチ（抑揚）のみを学習し、話者の声質を意図的に再現しません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本を再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)

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

## Speech Timing Target v1（日本語の時間伸縮）

`speech-timing-target-v1`は、`utautts-world-phrase`の日本語出力をモーラの中だけ時間伸縮するための目標モデルです（`internal/speechtiming/speech-timing-target-v1.safetensors`、bridgeに埋め込み）。抑揚モデルv10と同じ[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)（CV.夢前黎）と[みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/)のBASIC5000_0001-0600を、Montreal Forced Aligner（`japanese_mfa`）で整列して学習しました。

- 本モデルは発話ごとに正規化した包絡の動き（音の移り方の時間配分）だけを合成に使い、予測した包絡そのものや話者の声質は出力に含めません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本とMFA整列モデルを再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)、[licenses/MFA-Japanese-NOTICE.txt](../licenses/MFA-Japanese-NOTICE.txt)
- 学習: `cmd/tools/train-speech-timing`（gograd、[手順](../docs/model-training.md)）。同梱のv1は同じ特徴量・構成のPyTorch版で学習し、Go版は特徴量が一致し同等の検証誤差になることを確かめています。学習条件と検証の誤差はsafetensorsの`__metadata__`に記録しています

## Speech Timing Target v1（英語・中国語の時間伸縮）

`speech-timing-target-en-v1`と`speech-timing-target-zh-v1`は、英語・中国語の出力を時間伸縮するための目標モデルです（`internal/speechtiming/`、bridgeに埋め込み）。英語は抑揚モデル`frame-intonation-tcn-en-v1`と同じ[LibriTTS-R](https://www.openslr.org/141/)のtrain-clean-100の発話（MFA `english_us_arpa`で整列）、中国語は`tone-intonation-zh-v1`と同じ[AISHELL-3](https://www.openslr.org/93/)（PaddleSpeechの声調付き整列）で学習しました。

- 本モデルは発話ごとに正規化した包絡の動きだけを合成に使い、予測した包絡そのものや話者の声質は出力に含めません
- 重みは同じ学習元の抑揚モデルと同じライセンスで配布します。英語はCC BY 4.0、中国語はApache License 2.0です。UtauTTSはコーパスの音声・台本と整列データを再配布しません
- 英語の通知: [licenses/SPEECH-TIMING-TARGET-EN-V1.txt](../licenses/SPEECH-TIMING-TARGET-EN-V1.txt)、[licenses/LibriTTS-R-NOTICE.txt](../licenses/LibriTTS-R-NOTICE.txt)、[licenses/MFA-English-ARPA-NOTICE.txt](../licenses/MFA-English-ARPA-NOTICE.txt)、ライセンス本文: [licenses/CC-BY-4.0.txt](../licenses/CC-BY-4.0.txt)
- 中国語の通知: [licenses/SPEECH-TIMING-TARGET-ZH-V1.txt](../licenses/SPEECH-TIMING-TARGET-ZH-V1.txt)、[licenses/AISHELL-3-NOTICE.txt](../licenses/AISHELL-3-NOTICE.txt)、[licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt](../licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt)、ライセンス本文: [licenses/APACHE-2.0.txt](../licenses/APACHE-2.0.txt)

## モデルの記録

モデルJSONの`id`、`display_name`、`license`、`license_notices`、`provenance`、`training`、`metrics`に、学習元コーパス、ライセンス、学習条件、評価指標を記録します。`license_notices`は使用した各データの通知を列挙します。配布物にはモデルJSONと通知を含めます。
