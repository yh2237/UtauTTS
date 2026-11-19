# 抑揚モデルのライセンス

同梱モデルの出典とライセンス通知です。利用条件の原文は各配布元の規約に従います。UtauTTS全体のライセンスは [ライセンスの適用範囲](../LICENSE-SCOPE.md) を参照してください。

## 同梱モデル

| 言語 | 既定モデル |
| --- | --- |
| 日本語 | `frame-intonation-tcn-v12` |
| 英語 | `reference/`（Kokoro-82M） |
| 中国語 | `reference/kokoro-zh/`（Kokoro-82M-v1.1-zh） |

日本語の抑揚モデルはモーラ長を予測せず、指定した基準値と言語別の時間規則（「ん」0.9倍、「ー」1.2倍など）で決めます。英語と中国語の参照照合では、モーラ長と抑揚をKokoroの読みから決めます。学習と評価の手順は [モデルの学習](../docs/model-training.md) を参照してください。

## Frame Intonation TCN v12（既定）

`frame-intonation-tcn-v12` は、[Irodori-TTS](https://huggingface.co/Aratako/Irodori-TTS-v4.1-Small)（MIT）にBASIC5000とUtauTTS用の日常文を読ませた音声と、各種コーパスの音声からF0ヘッドを学習したモデルです。教師F0はオクターブ誤推定を前後の値に合わせて折り返し、音量の小さいフレームを除いて作成しています。

アクセント特徴とモーラの予測には、[つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)（CV.夢前黎）と [みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/) のBASIC5000_0001-0600で学習した基準モデル（v10）を使います（基準モデルはモデルJSONに埋め込み済み）。

v10の曲線は混ぜず、F0ヘッドの曲線のみを自然の振れ幅で使い、全体を50セント下げます。エネルギーヘッドでモーラの音量も0.75〜1.3倍に調整します。

- 相対ピッチ（抑揚）と音量を学習しており、話者の声質は意図的に再現しません
- 重みはMIT Licenseで配布します。教師の音声・台本・音声合成モデルは再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)、[licenses/MFA-Japanese-NOTICE.txt](../licenses/MFA-Japanese-NOTICE.txt)、[licenses/IRODORI-TEACHER-NOTICE.txt](../licenses/IRODORI-TEACHER-NOTICE.txt)

## English Reference Models（英語）

英語は `reference/` のKokoro-82M（声は `af_heart`）で文を読み、WavLM base+の特徴量で音源の録音から近いフレームを選んで合成します。仕組みは [音声合成の仕組み](../docs/how-utautts-speaks.md#9-参照照合) を参照してください。Kokoroの音声自体は出力に使いません。

- Kokoro-82Mとmisakiの英語辞書はApache License 2.0、WavLM base+（`microsoft/wavlm-base-plus`）はMIT Licenseです。いずれも配布元から変更せずに同梱します
- 通知: [licenses/REFERENCE-MODELS.txt](../licenses/REFERENCE-MODELS.txt)、ライセンス本文: [licenses/APACHE-2.0.txt](../licenses/APACHE-2.0.txt)、[licenses/WAVLM-MIT.txt](../licenses/WAVLM-MIT.txt)
- 取得: リポジトリでは `tools/fetch-reference-models.ps1`（Linux・macOSは `.sh`）でバージョンとSHA-256を固定して取得します。WavLMは配布元にsafetensorsがないため取得時に変換します

## Chinese Reference Model（中国語）

中国語の参照には `reference/kokoro-zh/` のKokoro-82M-v1.1-zh（声は `zf_001`）を使います。英語と同じく音源のフレームから合成し、漢字はピンインに変換して声母と韻母を注音符号にしたうえで参照TTSへ渡します。Kokoroの音声自体は出力に使いません。

- Kokoro-82M-v1.1-zhはApache License 2.0です。配布元から変更せずに同梱します
- 通知: [licenses/CHINESE-REFERENCE-MODEL.txt](../licenses/CHINESE-REFERENCE-MODEL.txt)、ライセンス本文: [licenses/APACHE-2.0.txt](../licenses/APACHE-2.0.txt)
- 取得: 英語と同じ `tools/fetch-reference-models.ps1`（Linux・macOSは `.sh`）でバージョンとSHA-256を固定して取得します

## Spanish and Portuguese Reference Voices（スペイン語・ポルトガル語）

スペイン語・ポルトガル語は英語と同じKokoro-82Mのモデルを、声 `ef_dora`・`pf_dora`（`reference/ef_dora.pt`、`pf_dora.pt`）で使います。綴りから規則で音素記号に変換します。ライセンスと通知は英語と同じです。

## Speech Timing Target v1（日本語の時間伸縮）

`speech-timing-target-v1` は、`utautts-world-phrase` の日本語出力をモーラ内だけ時間伸縮するための目標モデルです（`UtauWLD/speechtiming/speech-timing-target-v1.safetensors`、実行ファイルに埋め込み）。v12の基準モデル（v10）と同じ [つくよみちゃんコーパス Vol.1 声優統計コーパス（JVSコーパス準拠）](https://tyc.rei-yumesaki.net/material/corpus/)（CV.夢前黎）と [みんなで作るJSUTコーパスbasic5000](https://tyc.rei-yumesaki.net/material/minnade-jsut/) のBASIC5000_0001-0600をMontreal Forced Aligner（`japanese_mfa`）で整列して学習しました。

- 発話ごとに正規化した包絡の動き（音の移り方の時間配分）だけを合成に使い、予測した包絡そのものや話者の声質は出力に含めません
- 重みはMIT Licenseで配布します。学習元コーパスの利用条件は配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本とMFA整列モデルを再配布しません
- 通知: [licenses/TSUKUYOMI-CORPUS.txt](../licenses/TSUKUYOMI-CORPUS.txt)、[licenses/MINNADE-JSUT-CORPUS.txt](../licenses/MINNADE-JSUT-CORPUS.txt)、[licenses/MFA-Japanese-NOTICE.txt](../licenses/MFA-Japanese-NOTICE.txt)
- 学習: `cmd/tools/train-speech-timing`（gograd、[手順](../docs/model-training.md)）。同梱のv1は同じ特徴量・構成のPyTorch版で学習しており、Go版で特徴量が一致し検証誤差が同等になることを確認しています。学習条件と検証誤差はsafetensorsの `__metadata__` に記録しています

## モデルの記録

モデルJSONの `id`、`display_name`、`license`、`license_notices`、`provenance`、`training`、`metrics` に、学習元コーパス、ライセンス、学習条件、評価指標を記録します。`license_notices` には使用した各データの通知を列挙し、配布物にはモデルJSONと通知を含めます。
