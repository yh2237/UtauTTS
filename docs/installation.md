# インストール

[GitHub Releases](https://github.com/yh2237/UtauTTS/releases)から環境と用途に合うZIPをダウンロードします。

| パッケージ | 用途 |
| --- | --- |
| `UtauTTS-win-x64.zip` | Windows x64でGUIとCLI |
| `UtauTTS-linux-x64.zip` | Linux x64でGUIとCLI |
| `UtauTTS-mac-arm64.zip` | Apple Silicon MacでGUIとCLI |
| `UtauTTS-Server-win-x64.zip` | Windows x64向けHTTP Server |
| `UtauTTS-Server-linux-x64.zip` | Linux x64向けHTTP Server |
| `UtauTTS-Server-mac-arm64.zip` | Apple Silicon Mac向けHTTP Server |

## Windows

1. `UtauTTS-win-x64.zip`を任意のフォルダへ展開します。
2. 展開先の`utautts.exe`を実行します。
3. 左側へ文章を入力して再生ボタンで合成を確認します。

ZIP内のファイルは同じ階層構造のまま使用してください。runtimeやモデルだけを移動すると合成できません。

## macOS

macOS版はApple Silicon（arm64）向けです。`UtauTTS-mac-arm64.zip`を展開し、展開先のフォルダ構成を変更せずに使用してください。

現在のmacOS版はAppleの署名・公証を行っていないため初回起動時に警告が表示されることがあります。公式GitHub Releasesからダウンロードしたファイルであることを確認したうえで展開先へ移動し、隔離属性を解除して起動します。

```bash
cd "/path/to/extracted-folder"
xattr -rc "utautts.app" tools runtime
open "utautts.app"
```

`/path/to/extracted-folder`は実際に展開したフォルダのパスへ置き換えてください。`xattr`はアプリ本体だけでなく、同梱のCLI・更新ツール・runtimeにも適用します。配布元が信頼できることを確認できないファイルでは、この操作を行わないでください。

## Linux

Linux GUI版にはシステムにインストールしたQt 6.5以降（Qt Quick、Qt Quick Controls、Qt Multimedia）と日本語フォントが必要です。QtはLinux ZIPへ同梱されません。Debian 13では次のパッケージ構成で確認しています。

```bash
sudo apt-get update
sudo apt-get install -y fontconfig fonts-noto-cjk \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

ZIPを展開して必要に応じて実行権限を付けます。

```bash
chmod +x utautts tools/* runtime/utautts-openjtalk-features runtime/utautts-worldline-bridge
./utautts
```

Qtやデスクトップ環境が異なるディストリビューションでは同等のQt Quick・Qt Multimediaパッケージを導入してください。

## 更新

GUI版のアプリ内アップデーターは、通常は安定版だけを更新候補にします。開発者モードを有効にすると「プレリリース版も確認する」が設定画面に現れ、これも有効にした場合だけプレリリース版を確認します。プレリリースも通常リリースと同じ`vMAJOR.MINOR.PATCH`形式で判定します。

更新では`voice/`、`Resamplers/`、`Wavtools/`、`config.ini`と、引き継ぎ可能なユーザーRenderer定義を保持します。v1.2.2からの更新では、旧`plugins/renderers/`の`plugin.json`を現行の`renderer/`配置へ移す処理も更新中に行われ、更新後の初回起動で設定の移行状態を記録します。互換性を使う場合はZIPを手動で上書きせず、アプリ内アップデーターを使用してください。

## ボイスバンクを追加する

実行ファイルと同じ階層の`voice`ディレクトリへ音源ごとにフォルダを分けて配置します。

「ファイル」→「音源フォルダを開く」から`voice`ディレクトリを開けます。

配置したらUtauTTSを再起動するか「ファイル」→「音源を再読込」を選択して再読み込みしてください。

GUI版には「足立レイ ver3.5.0」を初期音源として同梱しています。利用前に[ボイスバンクの利用条件](voicebank.md)と音源内の文書を確認してください。

## Server版

Windows

```powershell
.\utautts-server.exe --voice-dir ".\voice"
```

Linux

```bash
chmod +x utautts-server runtime/utautts-openjtalk-features runtime/utautts-worldline-bridge
./utautts-server --voice-dir ./voice
```

macOS

```bash
xattr -rc "utautts-server" runtime
./utautts-server --voice-dir ./voice
```

`http://127.0.0.1:8080/`でコンソールUIを開けます。

[UtauTTS Server](server.md)を確認してください。
