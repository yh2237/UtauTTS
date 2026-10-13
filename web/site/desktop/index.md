---
title: インストール
description: デスクトップ版の入手、起動、更新
---

# インストール

環境に合うZIPを[GitHub Releases](https://github.com/yh2237/UtauTTS/releases)からダウンロードしてください。展開したフォルダを好きなところにおいてショートカットとか作って下さい。

| ファイル | 対応環境 |
| --- | --- |
| `UtauTTS-win-x64.zip` | Windows（x64） |
| `UtauTTS-mac-arm64.zip` | macOS（Apple Silicon） |
| `UtauTTS-linux-x64.zip` | Linux（x64） |

名前に`Server`が付くZIPはHTTP API向けです（[UtauTTS Server](https://github.com/yh2237/UtauTTS/blob/main/docs/server.md)）。

展開したフォルダ内の構成は変更しないでください。

## Windows

1. `UtauTTS-win-x64.zip`を任意のフォルダへ展開します。
2. 展開したフォルダの`utautts.exe`を起動します。

## macOS

macOS版はAppleの署名・公証を受けていないため、初回起動時に警告が出ます。公式のGitHub Releasesから入手したZIPを展開し、次を実行してください。

```bash
cd "/path/to/extracted-folder"
xattr -rc "utautts.app" tools runtime
open "utautts.app"
```

`/path/to/extracted-folder`は、実際に展開したフォルダのパスに置き換えてください。

## Linux

Linux版にはQt 6.5以降（Qt Quick、Qt Quick Controls、Qt Multimedia）と日本語フォントが必要です。Debian 13では次のパッケージをインストールします。

```bash
sudo apt-get update
sudo apt-get install -y fontconfig fonts-noto-cjk \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

ZIPを展開したら、必要に応じて実行権限を付けて起動します。

```bash
chmod +x utautts tools/* runtime/utautts-openjtalk-features runtime/utautts-worldline-bridge
./utautts
```

ほかのディストリビューションでは、対応するQtパッケージをインストールしてください。

## 更新

新しいバージョンは起動時に通知されます。特にこだわりがなければ更新してください。

更新に失敗する場合は大抵UtauTTSのファイルを他のプロセスが掴んでいる場合が殆どだと思うので、エクスプローラとかを全部落としてもう一度試してください。
