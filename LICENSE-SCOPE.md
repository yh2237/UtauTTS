# ライセンスの適用範囲

UtauTTSのソースコードはMIT Licenseです。UtauTTSが配布する成果物のうち、第三者が権利を持つ部分には個別のライセンスと通知が適用されます。この文書には、どの部分にどのライセンス・通知が適用されるかをまとめています。各ライセンスの条件本文は`licenses/`と[第三者通知](./THIRD_PARTY_NOTICES.txt)に収録します。

## 本体

UtauTTSのオリジナルコードは[`LICENSE`](./LICENSE)のMIT Licenseです。

## 同梱モデル

同梱する日本語の抑揚モデルと日本語の時間伸縮の目標モデルの重みはMIT Licenseで配布します。英語のモデル（`frame-intonation-tcn-en-v1`・`speech-timing-target-en-v1`）の重みはCC BY 4.0、中国語のモデル（`tone-intonation-zh-v1`・`speech-timing-target-zh-v1`）の重みはApache License 2.0です。各モデルが参照する通知を`licenses/`へ収録し、次の通り対応させます。

| モデル | 通知 |
| --- | --- |
| `frame-intonation-tcn-v12` | `licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt`, `licenses/MFA-Japanese-NOTICE.txt`, `licenses/IRODORI-TEACHER-NOTICE.txt` |
| `frame-intonation-tcn-v11` | `licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt`, `licenses/MFA-Japanese-NOTICE.txt`, `licenses/IRODORI-TEACHER-NOTICE.txt` |
| `frame-intonation-tcn-v10` | `licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt`, `licenses/MFA-Japanese-NOTICE.txt` |
| `speech-timing-target-v1`（bridgeへ埋め込み） | `licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt`, `licenses/MFA-Japanese-NOTICE.txt` |
| `frame-intonation-tcn-v9.1-t` | `licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt` |
| `frame-intonation-tcn-en-v1`（CC BY 4.0） | `licenses/ENGLISH-FRAME-INTONATION-TCN-V1.txt`, `licenses/LibriTTS-R-NOTICE.txt`, `licenses/MFA-English-ARPA-NOTICE.txt`, `licenses/CC-BY-4.0.txt` |
| `tone-intonation-zh-v1`（Apache License 2.0） | `licenses/TONE-INTONATION-ZH-V1.txt`, `licenses/AISHELL-3-NOTICE.txt`, `licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt`, `licenses/APACHE-2.0.txt` |
| `speech-timing-target-en-v1`（bridgeへ埋め込み、CC BY 4.0） | `licenses/SPEECH-TIMING-TARGET-EN-V1.txt`, `licenses/LibriTTS-R-NOTICE.txt`, `licenses/MFA-English-ARPA-NOTICE.txt`, `licenses/CC-BY-4.0.txt` |
| `speech-timing-target-zh-v1`（bridgeへ埋め込み、Apache License 2.0） | `licenses/SPEECH-TIMING-TARGET-ZH-V1.txt`, `licenses/AISHELL-3-NOTICE.txt`, `licenses/PADDLESPEECH-AISHELL3-ALIGNMENT-NOTICE.txt`, `licenses/APACHE-2.0.txt` |

モデルの利用時は各モデルJSONに記載した重みのライセンスに従ってください。学習元コーパスの利用条件は、配布元が公開する原文に従います。UtauTTSはコーパスの音声・台本を再配布しません。各通知は出典の情報提供であり、規約の解釈や権利者の代弁は行いません。モデルごとの出典は[抑揚モデルのライセンス](./models/README.md)にあります。

## 同梱ボイスバンク

GUI版の初期音源「足立レイ ver3.5.0」はUtauTTS本体とは別の条件で配布します。出典と条件は[同梱音源](./docs/voicebank.md)を参照してください。Server版に初期音源はありません。

## 第三者コンポーネント

Qt、WORLD、Open JTalk、Go/Pythonの依存関係、外部FFmpegなどには、それぞれのライセンスと通知が適用されます。全文は[`licenses/`](./licenses/)、概要は[第三者通知](./THIRD_PARTY_NOTICES.txt)に収録します。互換処理で参照した公開実装は[第三者コードの出典](./docs/third-party-provenance.md)に記載しています。

## リリースパッケージ

- GUI版: モデル、Renderer、ランタイム、ライセンス文書、初期ボイスバンク、対象プラットフォームのGUI通知。
- Server版: モデル、Renderer、ランタイム、ライセンス文書。

共通収録物: [`LICENSE`](./LICENSE)、`LICENSE-SCOPE.md`、[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、[`licenses/`](./licenses/)、モデルの通知。GUI版は同梱ボイスバンクの文書も収録します。

利用条件が競合する場合は、各権利者が公開する原文ライセンスと配布条件を優先します。
