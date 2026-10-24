# ライセンスの適用範囲

UtauTTSのソースコードはMIT Licenseです。配布パッケージに含まれる第三者由来の成果物には、個別のライセンスと通知が適用されます。この文書では、各コンポーネントと適用ライセンスの対応関係をまとめています。各ライセンスの本文は `licenses/` および [第三者通知](./THIRD_PARTY_NOTICES.txt) に収録されています。

## 本体コード

UtauTTSのオリジナルコードは [`LICENSE`](./LICENSE)（MIT License）が適用されます。

## 同梱モデル

日本語の抑揚モデルおよび時間伸縮モデルの重みはMIT Licenseで配布しています。参照照合に使う`reference/`のモデル（Kokoro-82M、Kokoro-82M-v1.1-zh、misakiの英語辞書）はApache License 2.0、WavLM base+はMIT Licenseです。各モデルが参照するライセンス通知は `licenses/` に収録されています。

| モデル | 適用ライセンス / 通知ファイル |
| :--- | :--- |
| `frame-intonation-tcn-v12`（v10を内包） | `licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt`, `licenses/MFA-Japanese-NOTICE.txt`, `licenses/IRODORI-TEACHER-NOTICE.txt` |
| `speech-timing-target-v1`（実行ファイルに内包） | `licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt`, `licenses/MFA-Japanese-NOTICE.txt` |
| `reference/`（参照照合のモデル。Kokoro-82M、misakiの英語辞書） | Apache License 2.0 (`licenses/REFERENCE-MODELS.txt`, `licenses/APACHE-2.0.txt`) |
| `reference/wavlm/`（WavLM base+） | MIT License (`licenses/REFERENCE-MODELS.txt`, `licenses/WAVLM-MIT.txt`) |
| `reference/kokoro-zh/`（Kokoro-82M-v1.1-zh） | Apache License 2.0 (`licenses/CHINESE-REFERENCE-MODEL.txt`, `licenses/APACHE-2.0.txt`) |

モデルを利用する際は、各モデルJSONに記載されたライセンス条項に従ってください。学習元コーパスの利用条件は、各配布元の規約に準拠します。詳細は [抑揚モデルのライセンス](./models/README.md) を参照してください。

## 同梱ボイスバンク

GUI版の初期音源「足立レイ ver3.5.0」は、UtauTTS本体とは独立した独自の規約で配布されています。利用規約およびガイドラインは [同梱音源](./docs/voicebank.md) をご確認ください。

## 第三者コンポーネント

Qt、WORLD、Open JTalk、Go依存ライブラリ、外部FFmpeg等には、各開発元が定めるライセンスが適用されます。ライセンス全文は [`licenses/`](./licenses/)、概要は [第三者通知](./THIRD_PARTY_NOTICES.txt) に記載されています。また、互換処理にあたって参照した公開コードは [第三者コードの出典](./docs/third-party-provenance.md) を参照してください。

## リリースパッケージの構成

- **GUI版**: モデル、ランタイム、ライセンス文書、初期ボイスバンク、プラットフォーム固有の通知
- **Server版**: モデル、ランタイム、ライセンス文書

全パッケージ共通で [`LICENSE`](./LICENSE)、`LICENSE-SCOPE.md`、[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、[`licenses/`](./licenses/) を同梱しています。

※利用条件に競合がある場合は、各権利者が公開している原文ライセンスおよび配布条件が優先されます。
