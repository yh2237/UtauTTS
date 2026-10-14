# UtauTTS

UTAUボイスバンクの原音接続に、学習ベースのイントネーション調整を加えたTTS

**まだベータ版みたいなもんなのでゴリゴリ仕様が変わります。**

あとスター付けてくれるとめっちゃ喜びます

> ボイスバンクを使う前に、各音源の利用規約を確認してください。UtauTTSはボイスバンクの利用で生じた問題について責任を負いません。

## インストール

[GitHub Releases](https://github.com/yh2237/UtauTTS/releases) からお使いの環境・用途に合ったZIPファイルをダウンロードしてください。

| パッケージ | 内容 |
| :--- | :--- |
| `UtauTTS-win-x64.zip` | Windows x64向け（GUI / CLI / HTTP Server同梱） |
| `UtauTTS-linux-x64.zip` | Linux x64向け（GUI / CLI / HTTP Server同梱） |
| `UtauTTS-mac-arm64.zip` | Apple Silicon Mac向け（GUI / CLI / HTTP Server同梱） |
| `UtauTTS-Server-win-x64.zip` | Windows x64向け HTTP Server単体版 |
| `UtauTTS-Server-linux-x64.zip` | Linux x64向け HTTP Server単体版 |
| `UtauTTS-Server-mac-arm64.zip` | Apple Silicon Mac向け HTTP Server単体版 |

- **Windows**: ZIPを展開し、中にある `utautts.exe` を実行します。
- **Linux / macOS**: 実行権限の付与などが必要です。詳細は [インストール手順](docs/installation.md) を参照してください。

※GUI版には初期音源として「足立レイ ver3.5.0」を同梱しています（利用条件は [同梱音源](docs/voicebank.md) および音源付属のライセンス文書をご確認ください）。

## 基本的な使い方

1. テキスト入力欄に読み上げたい文章を入力します。
2. 画面下部の「基本編集」でイントネーション曲線や発音長を確認し、必要に応じて制御点や境界線をドラッグして調整します。
3. `Ctrl + Enter` または「再生」ボタンでプレビュー音声を生成・再生します。
4. 「ファイル」→「書き出し」→「WAVを保存...」で音声ファイルとして保存します。

追加音源は、展開先フォルダ直下の `voice/` にフォルダごと配置し、「ファイル」→「音源とプラグイン」→「音源を再読込」を実行してください。音源の配置やカード設定、操作の詳細は [GUIの使い方](docs/gui.md) をご覧ください。

合成設定は [GUIの使い方](docs/gui.md#カードごとの合成設定)、同梱モデルと学習元データは [抑揚モデルのライセンス](models/README.md) を参照してください。

## CLI & HTTP Server

### CLI
GUI版の `tools/utautts-cli.exe`（Linux / macOSは `tools/utautts-cli`）から直接実行できます。

```powershell
.\tools\utautts-cli.exe `
  --voicebank ".\voice\足立レイver3.5.0" `
  --text "こんにちは、今日はいい天気です。" `
  --prosody frame-intonation-tcn-v12 `
  --out ".\out.wav"
```

### HTTP Server
GUI版に同梱の `utautts-server.exe`（Linux / macOSは `utautts-server`）を実行します。GUIとは独立して起動可能です。

```powershell
.\utautts-server.exe
```

起動後、ブラウザで `http://127.0.0.1:8080/` を開くとWebコンソールUIが利用できます。全オプションは [コマンドライン](docs/cli.md)、API仕様は [UtauTTS Server](docs/server.md) を参照してください。

## トラブルシューティング

うまく動かない場合は [トラブルシューティング](docs/troubleshooting.md) をご確認ください。解決しない場合は GitHub Issues または（[@2237yh](https://x.com/2237yh)）までお気軽にお問い合わせください。

## ドキュメント一覧

機能解説や開発者向け資料は [ドキュメント一覧](docs/README.md) にまとめています。

## 謝辞
- [アアアアアアア（@a7_riri）](https://x.com/a7_riri)
- [siyukatu（@siyukat）](https://x.com/siyukat)
- [WhosThat（@WndertheTree）](https://x.com/WndertheTree)

## ライセンス

UtauTTSのソースコードは [MIT License](./LICENSE) です。同梱モデル、ボイスバンク、コーパス、OpenUtau / WORLD由来ファイル、Qtなどの第三者コンポーネントには個別のライセンスが適用されます。詳細は [ライセンスの適用範囲](./LICENSE-SCOPE.md)、[第三者通知](./THIRD_PARTY_NOTICES.txt)、`licenses/` および同梱文書をご確認ください。
