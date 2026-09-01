# インストール

## パッケージを選ぶ

[GitHub Releases](https://github.com/yh2237/UtauTTS/releases)から環境と用途に合うZIPをダウンロードします。

| パッケージ | 用途 |
| --- | --- |
| `UtauTTS-win-x64.zip` | Windows x64でGUIとCLI |
| `UtauTTS-linux-x64.zip` | Linux x64でGUIとCLI |
| `UtauTTS-Server-win-x64.zip` | Windows x64向けHTTP Server |
| `UtauTTS-Server-linux-x64.zip` | Linux x64向けHTTP Server |

## Windows

1. `UtauTTS-win-x64.zip`を任意のフォルダへ展開します。
2. 展開先の`utautts.exe`を実行します。
3. 左側へ文章を入力して再生ボタンで合成を確認します。

ZIP内のファイルは同じ階層構造のまま使用してください。runtimeやモデルだけを移動すると合成できません。

## Linux

Linux GUI版にはQt 6.5以降と日本語フォントが必要です。Debian 13では次のパッケージ構成で確認しています。

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

`http://127.0.0.1:8080/`でコンソールUIを開けます。

[UtauTTS Server](server.md)を確認してください。
