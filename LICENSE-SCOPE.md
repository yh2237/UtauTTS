# ライセンスの適用範囲

UtauTTSはOSSのフリーソフトウェアとして公開しています。ただしリポジトリに含まれる学習済みモデル、音源、辞書、アイコン、データ、外部ライブラリのすべてがMIT Licenseの対象になるわけではありません。

## UtauTTS の本体コード

UtauTTSのオリジナルコードは、リポジトリ直下の[`LICENSE`](./LICENSE)に記載したMIT Licenseの対象です。MIT Licenseは、第三者が著作権を持つコード、データ、モデル、音源の利用条件を変更しません。

## 学習済みモデル

- 現在`models/`に同梱しているJSONモデルは、JSUT日本語音声コーパスの音声を使って学習したモデルです。
- 同梱モデルの利用・改変・再配布条件は、学術研究、非商用研究、個人利用に限られます。商用利用にはJSUT権利者の事前許諾が必要です。
- モデル自体をCreative Commons Attribution-ShareAlike 4.0 Internationalとして配布しているわけではありません。
- 詳細は[`models/README.md`](./models/README.md)、[`licenses/PROSODY-MODELS.txt`](./licenses/PROSODY-MODELS.txt)、[`licenses/JSUT-DATA-AND-LABELS.txt`](./licenses/JSUT-DATA-AND-LABELS.txt)を参照してください。
- JSUTの元音声、BASIC5000原典全体、jsut-labelの全データは、リリースパッケージに同梱していません。GUIに埋め込まれるBASIC5000の選択300文については、次の文章セットの項を参照してください。モデルの条件は、これらの上流データの権利を移転するものではありません。

## 抑揚調整用の文章セット

GUIに埋め込まれる`qt/prosody-prompts-ja-v1.json`は、UtauTTSが作成した10文と、JSUT BASIC5000から選択した300文を含む混在データです。ファイル全体をMIT Licenseと表示してはいけません。構成要素ごとの条件はJSONの`sources`メタデータと[`licenses/JSUT-DATA-AND-LABELS.txt`](./licenses/JSUT-DATA-AND-LABELS.txt)に記載しています。

選択したBASIC5000の文章を含むリソースや、そこから作成した教師データを再配布する場合は、田中コーパス、Wikipedia、JSUT独自文それぞれの出典表示とライセンス条件を維持してください。

## ボイスバンク

GUI版に同梱する`voice/`の足立レイ音源は、メカニカルガール公式配布物に含まれる音源であり、UtauTTS本体のMIT Licenseとは別の条件に従います。リポジトリでは`voice/README.md`と`docs/voicebank.md`、配布物では`voice/`内の公式文書と最新の公式ガイドラインを確認してください。Server版にはこの音源を同梱しません。商用・収益目的など、配布元への確認が必要な用途は、許諾を得てから行ってください。

## その他の第三者コンポーネント

OpenUtau/WORLD/WORLDLINE、Qt、FFmpeg、Breeze Icons、Open JTalk、Go/Pythonの依存関係などは、それぞれのライセンスと通知を維持します。対象と配布時のファイルは[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、`THIRD_PARTY_NOTICES-*`、[`licenses/`](./licenses/)および各コンポーネントの同梱文書に記載しています。

`icons/`のアプリケーションアイコン、`qt/assets/icons/`のUIアイコンなど、画像・アイコンの出所とライセンスは個別に確認してください。KDE Breeze由来のUIアイコンには、同梱されるBreezeの通知とライセンスが適用されます。

## リリースに含める範囲

リリースビルドは、GUI版とServer版それぞれに必要なモデル、Renderer、ランタイム、ライセンス文書を明示的にコピーします。GUI版には初期音源を同梱しますが、Server版には同梱しません。`data/`、`out/`、`.tmp-*`、旧版の`release/`などの開発用・学習用ディレクトリは、配布物に含めません。

各リリースパッケージには、少なくとも本体の[`LICENSE`](./LICENSE)、この文書、[`THIRD_PARTY_NOTICES.txt`](./THIRD_PARTY_NOTICES.txt)、`licenses/`、モデルの個別文書を含めます。GUI版には対象プラットフォームのGUI固有通知と同梱音源の公式文書も含めます。Server版にはGUI固有通知と同梱音源を含めません。

この文書は適用範囲の要約です。具体的な利用条件に矛盾がある場合は、各権利者の原文ライセンス・配布条件および同梱通知を優先してください。
