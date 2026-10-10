---
title: インストール
description: デスクトップ版の入手、起動、更新
---

# インストール

デスクトップ版は、[GitHubのリリース](https://github.com/yh2237/UtauTTS/releases)からZIPをダウンロードして使います。インストーラーはありません。ZIPを展開したフォルダから起動します。

| ファイル | 対応環境 |
| --- | --- |
| `UtauTTS-win-x64.zip` | Windows（x64） |
| `UtauTTS-mac-arm64.zip` | macOS（Apple Silicon） |
| `UtauTTS-linux-x64.zip` | Linux（x64） |

名前に`Server`が付くZIPは、HTTPで音声合成を使うためのServer版です（[UtauTTS Server](https://github.com/yh2237/UtauTTS/blob/main/docs/server.md)）。

展開したフォルダの中身は、構成を変えずに使ってください。`runtime`フォルダやモデルだけを別の場所へ移すと、音声を合成できなくなります。

## Windows

1. `UtauTTS-win-x64.zip`を好きなフォルダへ展開します。
2. 展開したフォルダの`utautts.exe`を起動します。
3. 左のカードに文章を入れ、再生ボタンで音声を確かめます。

## macOS

macOS版はApple Silicon向けです。Appleの署名と公証を受けていないため、初回の起動時に警告が出ることがあります。公式のGitHubリリースからダウンロードしたファイルであることを確かめてから、ターミナルで次のコマンドを実行し、ダウンロードしたファイルに付く隔離属性を外します。

```bash
cd "/path/to/extracted-folder"
xattr -rc "utautts.app" tools runtime
open "utautts.app"
```

`/path/to/extracted-folder`は、実際に展開したフォルダのパスに置き換えてください。入手元を確かめられないファイルには、この操作をしないでください。

## Linux

Linux版は、システムにインストールしたQt 6.5以降（Qt Quick、Qt Quick Controls、Qt Multimedia）と日本語フォントを使います。Debian 13では、次のパッケージをインストールします。

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

ほかのディストリビューションでは、同じ機能のQtパッケージをインストールしてください。

## 更新

新しいバージョンが出ると、アプリの更新ツールで更新できます。更新では、`voice`、`Resamplers`、`Wavtools`、`Dependencies`の各フォルダと、設定を保存した`config.ini`を引き継ぎます。ZIPを手で上書きせず、更新ツールを使ってください。

更新の候補になるのは、通常は正式版だけです。設定で「プレリリース版も確認する」を有効にすると、`v1.5.0-beta.1`のような試験版も候補に入ります。
