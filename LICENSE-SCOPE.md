# ライセンスの適用範囲

配布パッケージに含まれる各コンポーネントと適用ライセンスの対応です。各ライセンスの本文は [`licenses/`](./licenses/) と [第三者通知](./THIRD_PARTY_NOTICES.txt) に収録しています。

## 本体コード

UtauTTS本体のソースコードには [`LICENSE`](./LICENSE)（MIT License）が適用されます。

## 同梱モデル

モデルごとの適用ライセンスと通知ファイル（`licenses/` 内）は次のとおりです。

| モデル | 適用ライセンス / 通知ファイル |
| :--- | :--- |
| `frame-intonation-tcn-v12`（v10を内包） | MIT License (`licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt`, `licenses/MFA-Japanese-NOTICE.txt`, `licenses/IRODORI-TEACHER-NOTICE.txt`) |
| `speech-timing-target-v1`（実行ファイルに内包） | MIT License (`licenses/TSUKUYOMI-CORPUS.txt`, `licenses/MINNADE-JSUT-CORPUS.txt`, `licenses/MFA-Japanese-NOTICE.txt`) |
| `reference/`（参照照合モデル: Kokoro-82M、misakiの英語辞書） | Apache License 2.0 (`licenses/REFERENCE-MODELS.txt`, `licenses/APACHE-2.0.txt`) |
| `reference/wavlm/`（WavLM base+） | MIT License (`licenses/REFERENCE-MODELS.txt`, `licenses/WAVLM-MIT.txt`) |
| `reference/kokoro-zh/`（Kokoro-82M-v1.1-zh） | Apache License 2.0 (`licenses/CHINESE-REFERENCE-MODEL.txt`, `licenses/APACHE-2.0.txt`) |
| `internal/g2p/lexicon/cmudict.dict.gz`（英語の基本辞書） | CMUdict license (`licenses/Go/CMUDICT-LICENSE.txt`) |
| `internal/g2p/en/english-g2p.bin`（英語の辞書外語用GRUモデル: g2p-en） | Apache License 2.0 (`licenses/G2P-EN-NOTICE.txt`, `licenses/APACHE-2.0.txt`) |
| `internal/g2p/lexicon/librispeech.dict.gz`（英語の補助辞書: LibriSpeech） | Public domain (`licenses/LIBRISPEECH-LEXICON-NOTICE.txt`) |

モデルを利用する際は、各モデルJSONのライセンス条項に従ってください。学習元コーパスの利用条件は各配布元の規約に従います。詳細は [抑揚モデルのライセンス](./models/README.md) を参照してください。

## 同梱ボイスバンク

GUI版の初期音源「足立レイ ver3.5.0」には独自の利用規約があります。利用規約とガイドラインは [同梱音源](./docs/voicebank.md) を確認してください。

## 第三者コンポーネント

Qt、WORLD、Open JTalk、Go依存ライブラリ、外部FFmpegなどには各開発元のライセンスが適用されます。Web版の参照照合推論に使うONNX Runtime WebはMIT Licenseです。ライセンス全文は [`licenses/`](./licenses/)、概要は [第三者通知](./THIRD_PARTY_NOTICES.txt)、互換処理で参照した公開コードは [第三者コードの出典](./docs/third-party-provenance.md) を参照してください。

`licenses/Qt/GPL-3.0.txt` は、Qtが使うLGPLv3がGPLv3の条項を参照しているため同梱しています。UtauTTS本体のMIT Licenseは変わりません。

## リリースパッケージの構成

- GUI版: モデル、ランタイム、ライセンス文書、初期ボイスバンク、プラットフォーム固有の通知
- Server版: モデル、ランタイム、ライセンス文書
- Web版: ブラウザで動くWasmと参照照合モデル

全パッケージ共通で [`LICENSE`](./LICENSE)、`LICENSE-SCOPE.md`、[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、[`licenses/`](./licenses/) を同梱しています。

※利用条件に競合がある場合は、各権利者が公開している原文ライセンスと配布条件が優先されます。
