# ライセンスの適用範囲

UtauTTSのソースコードはMIT Licenseで公開しています。学習済みモデル、ボイスバンク、文章データ、アイコン、外部ライブラリなど、第三者が権利を持つ成果物には個別の利用条件が適用されます。

## UtauTTSのソースコード

UtauTTSのオリジナルコードは[`LICENSE`](./LICENSE)のMIT Licenseに従います。このライセンスは、第三者が権利を持つコード、データ、モデル、音源の利用条件を変更しません。

## 学習済みモデル

現在配布している`models/`の公式JSONモデルは、JSUT日本語音声コーパスの音声を使って学習しています。

- 利用、改変、再配布は、学術研究、非商用研究、個人利用に限られます。
- 商用利用にはJSUT権利者の事前許諾が必要です。
- モデル自体はCreative Commons Attribution-ShareAlike 4.0 Internationalとして配布していません。
- 詳細な条件と出典は、[`models/README.md`](./models/README.md)、[`licenses/PROSODY-MODELS.txt`](./licenses/PROSODY-MODELS.txt)、[`licenses/JSUT-DATA-AND-LABELS.txt`](./licenses/JSUT-DATA-AND-LABELS.txt)を確認してください。

JSUTの元音声、BASIC5000全体、jsut-labelの全データは配布物に含まれません。モデルの利用条件は、これらの上流データの権利を移転するものではありません。

## ボイスバンク

GUI版には、メカニカルガール公式配布の「足立レイ UTAU音源 ver3.5.0」を同梱しています。この音源はUtauTTS本体のMIT Licenseの対象外です。利用条件は[同梱ボイスバンクの案内](./docs/voicebank.md)、音源に付属する文書、[公式ガイドライン](https://mechanicalgirl.jp/guidelines/)を確認してください。Server版にはこの音源を同梱していません。

## その他の第三者コンポーネント

OpenUtau、WORLD、Qt、FFmpeg、Breeze Icons、Open JTalk、Go/Pythonの依存関係などには、それぞれのライセンスと通知が適用されます。詳細は[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、[Windows GUI通知](./THIRD_PARTY_NOTICES-WINDOWS-GUI.txt)、[macOS GUI通知](./THIRD_PARTY_NOTICES-MACOS-GUI.txt)、[`licenses/`](./licenses/)、各コンポーネントの同梱文書を確認してください。

アプリケーションアイコンとUIアイコンも、出典ごとに利用条件が異なります。`icons/`および`qt/assets/icons/`のアイコンを再配布・改変する場合は、対応する通知とライセンスを確認してください。

## リリースパッケージ

GUI版とServer版には、それぞれの実行に必要なモデル、Renderer、ランタイム、ライセンス文書を含めます。GUI版には初期ボイスバンクを同梱しますが、Server版には同梱しません。

各パッケージには、少なくとも[`LICENSE`](./LICENSE)、この文書、[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、`licenses/`、モデルの個別文書を含めます。GUI版には対象プラットフォームのGUI固有通知と、同梱ボイスバンクの公式文書も含めます。Server版にはGUI固有通知と同梱ボイスバンクを含めません。

具体的な利用条件に矛盾がある場合は、各権利者が公開する原文ライセンス、配布条件、同梱通知を優先してください。
