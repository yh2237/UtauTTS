# インストールと起動

[GitHub Releases](https://github.com/yh2237/UtauTTS/releases) から、環境と用途に合うZIPをダウンロードしてください。

| パッケージ | 用途 |
| :--- | :--- |
| `UtauTTS-win-x64.zip` | Windows x64向けGUI・CLI・HTTP Server |
| `UtauTTS-linux-x64.zip` | Linux x64向けGUI・CLI・HTTP Server |
| `UtauTTS-mac-arm64.zip` | Apple Silicon Mac向けGUI・CLI・HTTP Server |
| `UtauTTS-Server-win-x64.zip` | Windows x64向けHTTP Server単体版 |
| `UtauTTS-Server-linux-x64.zip` | Linux x64向けHTTP Server単体版 |
| `UtauTTS-Server-mac-arm64.zip` | Apple Silicon Mac向けHTTP Server単体版 |

---

## Windows

1. `UtauTTS-win-x64.zip` を任意のフォルダへ展開します。
2. 展開先の `utautts.exe` を実行します。

> **注意**: ZIP内のフォルダ構成（`runtime/` や `models/` など）は変えないでください。変えると合成エンジンやモデルを読み込めなくなります。

外部のFFmpegを使うときは、設定画面でQt Multimedia用のプラグインとコーデックのフォルダを指定します。FFmpegのパスが未設定のときは、起動時に環境変数 `UTAUTTS_FFMPEG_PATH`、`FFMPEG_PATH`、`FFMPEG_DIR`、`FFMPEG_ROOT` を順に確認し、最初に見つかったパスを保存します。

---

## macOS（Apple Silicon）

署名と公証が済んでいないため、初回の起動時にセキュリティの警告が出る場合があります。

1. `UtauTTS-mac-arm64.zip` を展開します。
2. ターミナルで展開先に移動し、隔離属性（quarantine）を外して起動します。
   ```bash
   cd "/path/to/extracted-folder"
   xattr -rc "utautts.app" tools runtime
   open "utautts.app"
   ```

---

## Linux（x64）

Linux GUI版にはQt 6.5以降（Qt Quick, Qt Quick Controls, Qt Multimedia）と日本語フォントが必要です。

Debian / Ubuntu系での導入例:
```bash
sudo apt-get update
sudo apt-get install -y fontconfig fonts-noto-cjk \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

ZIPを展開したら、実行ファイルに実行権限を付けて起動します。
```bash
chmod +x utautts tools/* runtime/utautts-openjtalk-features runtime/utautts-worldline-bridge
./utautts
```

---

## 更新

更新にはアプリ内のアップデーターを使ってください。ZIPを手で上書きしないでください。更新では `voice/`、`Resamplers/`、`Wavtools/`、`Dependencies/`、`config.ini` と、引き継げるユーザーのRenderer定義を残します。Renderer定義は `renderer/<id>/renderer.json` に置いてください。

---

## ボイスバンクの追加

実行ファイルと同じ階層の `voice/` に、音源フォルダをそのまま置いてください（macOS版は `utautts.app` と同じ階層の `voice/`）。

```text
voice/
  └── 音源名/
        ├── oto.ini
        └── *.wav
```

- 置いたあとは、アプリを再起動するか「音源を再読込」を実行すると読み込まれます。
- 配布ZIPは「音源を追加...」で選ぶと `voice/` 直下へ展開して登録します。

GUI版には「足立レイ ver3.5.0」を同梱しています。利用条件は [同梱音源](voicebank.md) を確認してください。

---

## HTTP Serverの起動

GUI版でもServer単体版でも、展開先のフォルダで次のコマンドを実行します。GUIとは別に起動できます。

- Windows:
  ```powershell
  .\utautts-server.exe --voice-dir ".\voice"
  ```
- Linux:
  ```bash
  chmod +x utautts-server runtime/utautts-openjtalk-features runtime/utautts-worldline-bridge
  ./utautts-server --voice-dir ./voice
  ```
- macOS:
  ```bash
  xattr -rc "utautts-server" runtime
  ./utautts-server --voice-dir ./voice
  ```

起動後、ブラウザで `http://127.0.0.1:8080/` を開くとWebコンソールが開きます。詳細は [UtauTTS Server](server.md) を参照してください。
