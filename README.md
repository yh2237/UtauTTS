# UtauTTS

UTAUボイスバンクの原音接続に、学習ベースのイントネーション調整を加えたTTS

**まだベータ版みたいなもんなのでゴリゴリ仕様が変わります。**

> ボイスバンクを使う前に、各音源の利用規約を確認してください。UtauTTSはボイスバンクの利用で生じた問題について責任を負いません。

## インストール

[GitHub Releases](https://github.com/yh2237/UtauTTS/releases)から環境と用途に合うZIPをダウンロードします。

| パッケージ | 用途 |
| --- | --- |
| `UtauTTS-win-x64.zip` | Windows x64向けGUIとCLI |
| `UtauTTS-linux-x64.zip` | Linux x64向けGUIとCLI |
| `UtauTTS-mac-arm64.zip` | Apple Silicon Mac向けGUIとCLI |
| `UtauTTS-Server-win-x64.zip` | Windows x64向けHTTP Server |
| `UtauTTS-Server-linux-x64.zip` | Linux x64向けHTTP Server |
| `UtauTTS-Server-mac-arm64.zip` | Apple Silicon Mac向けHTTP Server |

Windows版はZIPを展開して`utautts.exe`を実行します。Linux版とmacOS版の準備は[インストール](docs/installation.md)を参照してください。

GUI版には「足立レイ ver3.5.0」を同梱しています。利用条件は[同梱音源](docs/voicebank.md)と音源内の文書を確認してください。

## 使い方

1. 文章欄へ読み上げる文を入力する
2. 下部の「基本編集」でイントネーションと発音の長さを確認し、必要なら点や境界線を動かす
3. `Ctrl+Enter`または再生ボタンで音声を確認する
4. 「ファイル」→「書き出し」→「WAVを保存...」で保存する

音源は実行ファイルと同じ階層の`voice`へフォルダごと置き、「ファイル」→「音源とプラグイン」→「音源を再読込」で読み込みます。音源の配置、カードごとの設定、編集の操作は[GUIの使い方](docs/gui.md)にあります。

既定の抑揚モデルは`frame-intonation-tcn-v11`、Rendererは`utautts-world-phrase`です。英語・中国語のカードでは各言語のモデルへ自動で切り替わります。同梱モデルと学習元は[抑揚モデルのライセンス](models/README.md)にあります。

## CLIとHTTP Server

CLIはGUI版の`tools/utautts-cli.exe`（Linux／macOSは`tools/utautts-cli`）です。

```powershell
.\UtauTTS\tools\utautts-cli.exe `
  --voicebank ".\UtauTTS\voice\足立レイver3.5.0" `
  --text "こんにちは、今日はいい天気です。" `
  --prosody frame-intonation-tcn-v11 `
  --out ".\out.wav"
```

Server版を起動すると`http://127.0.0.1:8080/`でコンソールUIを使えます。

```powershell
.\UtauTTS-Server\utautts-server.exe --voice-dir ".\UtauTTS-Server\voice"
```

全オプションは[コマンドライン](docs/cli.md)、APIは[UtauTTS Server](docs/server.md)を参照してください。

## うまく動かないとき

[トラブルシューティング](docs/troubleshooting.md)をご覧ください。

解決しない場合はIssueを送るか[@2237yh](https://x.com/2237yh)に直接DMを送ってください。

## ドキュメント

利用方法、仕組み、開発者向けの資料は[ドキュメント一覧](docs/README.md)にまとめています。

## 謝辞

- [アアアアアアア（@a7_riri）](https://x.com/a7_riri)
- [siyukatu（@siyukat）](https://x.com/siyukat)
- [WhosThat（@WndertheTree）](https://x.com/WndertheTree)

## ライセンス

UtauTTSのソースコードは[MIT License](./LICENSE)です。同梱モデル、ボイスバンク、文章データ、OpenUtau/WORLD由来ファイル、Qtなどの第三者コンポーネントには個別の利用条件があります。詳細は[ライセンスの適用範囲](./LICENSE-SCOPE.md)、[第三者通知](./THIRD_PARTY_NOTICES.txt)、`THIRD_PARTY_NOTICES-*`、`licenses/`、各同梱文書を確認してください。
