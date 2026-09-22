# ライセンスの適用範囲

UtauTTSのソースコードはMIT Licenseで公開しています。学習済みモデル、ボイスバンク、文章データ、外部ライブラリなど、第三者が権利を持つ成果物には個別の利用条件が適用されます。


## UtauTTSのソースコード

UtauTTSのオリジナルコードは[`LICENSE`](./LICENSE)のMIT Licenseです。第三者のコード、データ、モデル、音源には、それぞれの配布条件を適用します。

## 学習済みモデル

同梱する日本語モデル: `frame-intonation-v9-t`は[つくよみちゃんコーパス Vol.1](https://tyc.rei-yumesaki.net/material/corpus/)、`frame-intonation-v9-k`は[Kokoro Speech Dataset](https://github.com/kaiidams/Kokoro-Speech-Dataset)で学習した単一話者モデルです。

- `frame-intonation-v9-t`: 音声は商用・非商用可、クレジット必須。音声そのものの再配布は不可。詳細: [`licenses/TSUKUYOMI-CORPUS.txt`](./licenses/TSUKUYOMI-CORPUS.txt)
- `frame-intonation-v9-k`: 音声はパブリックドメイン。詳細: [`licenses/KOKORO-SPEECH-DATASET.txt`](./licenses/KOKORO-SPEECH-DATASET.txt)
- モデル重みはMIT Licenseで配布しますが、利用時は学習元コーパスの条件にも従ってください。

`english-intonation-v1`はUtauTTS用の係数モデルで、MIT Licenseで配布します。詳細: [`licenses/ENGLISH-INTONATION-V1.txt`](./licenses/ENGLISH-INTONATION-V1.txt)

## ボイスバンク

GUI版の初期音源: メカニカルガール公式配布「足立レイ UTAU音源 ver3.5.0」。音源の利用条件: [同梱ボイスバンクの案内](./docs/voicebank.md)、音源内の文書、[公式ガイドライン](https://mechanicalgirl.jp/guidelines/)。Server版の初期音源: なし。

## その他の第三者コンポーネント

Qt、WORLD、Open JTalk、Go/Pythonの依存関係、外部FFmpegには、それぞれのライセンスと通知を適用します。詳細: [`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、各プラットフォームのGUI通知、[`licenses/`](./licenses/)、各コンポーネントの文書。

アプリケーションアイコンと評価用ファイルは、本リポジトリのMIT Licenseで提供します。

## リリースパッケージ

GUI版: 実行に必要なモデル、Renderer、ランタイム、ライセンス文書、初期ボイスバンク、対象プラットフォームの通知。
Server版: 実行に必要なモデル、Renderer、ランタイム、ライセンス文書。

共通収録物: [`LICENSE`](./LICENSE)、`LICENSE-SCOPE.md`、[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、`licenses/`、モデルの個別文書。GUI版は同梱ボイスバンクの公式文書も収録します。

利用条件の優先順位: 各権利者が公開する原文ライセンス、配布条件、同梱通知。

OpenUtau互換処理とWORLDの出典、各ライセンス本文は[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)に記載しています。
