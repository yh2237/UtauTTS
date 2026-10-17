# UtauTTS

UTAUボイスバンクの原音接続に、学習ベースのイントネーション調整を加えたTTS

**まだベータ版みたいなもんなのでゴリゴリ仕様が変わります。**

あとスター付けてくれるとめっちゃ喜びます

> ボイスバンクを使う前に、各音源の利用規約を確認してください。UtauTTSはボイスバンクの利用で生じた問題について責任を負いません。

## インストール

[GitHub Releases](https://github.com/yh2237/UtauTTS/releases) から、環境と用途に合うZIPをダウンロードしてください。

| パッケージ | 内容 |
| :--- | :--- |
| `UtauTTS-win-x64.zip` | Windows x64向け（GUI / CLI / HTTP Server同梱） |
| `UtauTTS-linux-x64.zip` | Linux x64向け（GUI / CLI / HTTP Server同梱） |
| `UtauTTS-mac-arm64.zip` | Apple Silicon Mac向け（GUI / CLI / HTTP Server同梱） |
| `UtauTTS-Server-win-x64.zip` | Windows x64向け HTTP Server単体版 |
| `UtauTTS-Server-linux-x64.zip` | Linux x64向け HTTP Server単体版 |
| `UtauTTS-Server-mac-arm64.zip` | Apple Silicon Mac向け HTTP Server単体版 |

- Windows: ZIPを展開し、中の `utautts.exe` を実行します。
- Linux / macOS: 実行権限の付与などが必要です。[インストール手順](docs/installation.md) を参照してください。

GUI版には初期音源として「足立レイ ver3.5.0」を同梱しています。利用条件は [同梱音源](docs/voicebank.md) と音源付属のライセンス文書を確認してください。

## 基本的な使い方

1. 読み上げる文章を入力します。
2. 必要なら、画面下部の「基本編集」で制御点や境界線をドラッグして、イントネーションと発音の長さを調整します。
3. `Ctrl + Enter` か再生ボタンで合成して再生します。
4. 「ファイル」→「書き出し」→「WAVを保存...」で保存します。

追加の音源は展開先フォルダ直下の `voice/` にフォルダごと置き、「ファイル」→「音源とプラグイン」→「音源を再読込」を実行してください。

操作の詳細は [GUIの使い方](docs/gui.md)、合成設定は [カードごとの合成設定](docs/gui.md#カードごとの合成設定)、同梱モデルと学習元データは [抑揚モデルのライセンス](models/README.md) を参照してください。

## CLI & HTTP Server

### CLI
GUI版の `tools/utautts-cli.exe`（Linux / macOSは `tools/utautts-cli`）を使います。

```powershell
.\tools\utautts-cli.exe `
  --voicebank ".\voice\足立レイver3.5.0" `
  --text "こんにちは、今日はいい天気です。" `
  --prosody frame-intonation-tcn-v12 `
  --out ".\out.wav"
```

### HTTP Server
GUI版に同梱の `utautts-server.exe`（Linux / macOSは `utautts-server`）を実行します。GUIとは別に起動できます。

```powershell
.\utautts-server.exe
```

起動後、ブラウザで `http://127.0.0.1:8080/` を開くとWebコンソールを使えます。全オプションは [コマンドライン](docs/cli.md)、API仕様は [UtauTTS Server](docs/server.md) を参照してください。

## トラブルシューティング

うまく動かないときは [トラブルシューティング](docs/troubleshooting.md) を確認してください。解決しない場合は、GitHub Issues か [@2237yh](https://x.com/2237yh) までお気軽にお問い合わせください。

## ドキュメント一覧

機能の説明と開発者向けの資料は [ドキュメント一覧](docs/README.md) にあります。

## 謝辞
- [アアアアアアア（@a7_riri）](https://x.com/a7_riri)
- [siyukatu（@siyukat）](https://x.com/siyukat)
- [WhosThat（@WndertheTree）](https://x.com/WndertheTree)

## ライセンス

UtauTTSのソースコードは [MIT License](./LICENSE) です。同梱モデル、ボイスバンク、コーパス、OpenUtau / WORLD由来ファイル、Qtなどの第三者コンポーネントには個別のライセンスが適用されます。詳細は [ライセンスの適用範囲](./LICENSE-SCOPE.md)、[第三者通知](./THIRD_PARTY_NOTICES.txt)、`licenses/` と同梱文書を確認してください。
