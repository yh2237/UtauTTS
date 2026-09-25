# ライセンスの適用範囲

UtauTTSのソースコードはMIT Licenseです。UtauTTSが配布する成果物のうち、第三者が権利を持つ部分には個別のライセンスと通知が適用されます。この文書は、どの部分にどのライセンス・通知が適用されるかの入口です。各ライセンスの条件本文は`licenses/`と[第三者通知](./THIRD_PARTY_NOTICES.txt)に収録します。

## 本体

UtauTTSのオリジナルコードは[`LICENSE`](./LICENSE)のMIT Licenseです。

## 同梱モデル

同梱する抑揚モデルの重みはMIT Licenseで配布します。モデルごとに適用される通知は次の通りで、全文を`licenses/`へ収録します。

| モデル | 通知 |
| --- | --- |
| `frame-intonation-tcn-v9.1-t` | `licenses/FRAME-INTONATION-V9.1-T.txt` |
| `frame-intonation-tcn-v9-t` | `licenses/TSUKUYOMI-CORPUS.txt` |
| `frame-intonation-tcn-v9-k` | `licenses/KOKORO-SPEECH-DATASET.txt` |
| `english-intonation-v1` | `licenses/ENGLISH-INTONATION-V1.txt` |

モデルの利用時は、重みのMIT Licenseに加えて各通知の条件にも従ってください。モデルごとの出典と条件の要約は[抑揚モデルのライセンス](./models/README.md)にあります。

## 同梱ボイスバンク

GUI版の初期音源「足立レイ UTAU音源 ver3.5.0」はUtauTTS本体とは別の条件で配布します。出典と条件は[同梱音源](./docs/voicebank.md)を参照してください。Server版に初期音源はありません。

## 第三者コンポーネント

Qt、WORLD、Open JTalk、Go/Pythonの依存関係、外部FFmpegなどには、それぞれのライセンスと通知が適用されます。全文は[`licenses/`](./licenses/)、概要は[第三者通知](./THIRD_PARTY_NOTICES.txt)に収録します。互換処理で参照した公開実装は[第三者コードの出典](./docs/third-party-provenance.md)に記載しています。

## リリースパッケージ

- GUI版: モデル、Renderer、ランタイム、ライセンス文書、初期ボイスバンク、対象プラットフォームのGUI通知。
- Server版: モデル、Renderer、ランタイム、ライセンス文書。

共通収録物: [`LICENSE`](./LICENSE)、`LICENSE-SCOPE.md`、[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、[`licenses/`](./licenses/)、モデルの通知。GUI版は同梱ボイスバンクの文書も収録します。

利用条件が競合する場合は、各権利者が公開する原文ライセンスと配布条件を優先します。
