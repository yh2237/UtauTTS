# インストールと起動

[GitHub Releases](https://github.com/yh2237/UtauTTS/releases) から、お使いの環境と用途に合わせたZIPパッケージをダウンロードしてください。

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
2. 展開先にある `utautts.exe` を実行します。
3. 画面左側のカードに文章を入力し、再生ボタン（または `Ctrl + Enter`）で音声合成を確認します。

> **注意**: ZIP内のフォルダ構成（`runtime/` や `models/` など）は変更しないでください。配置を変更すると合成エンジンやモデルが正常に認識されなくなります。

外部FFmpegを使用する場合は、設定画面でQt Multimedia用プラグインとコーデックのフォルダを指定します。初回起動時は環境変数 `UTAUTTS_FFMPEG_PATH`、`FFMPEG_PATH`、`FFMPEG_DIR`、`FFMPEG_ROOT` を順に確認し、最初に見つかったパスを保存します。

---

## macOS（Apple Silicon）

macOS版は Apple Silicon（arm64）専用です。署名・公証が未完了のため、初回起動時にセキュリティ警告が出る場合があります。

1. `UtauTTS-mac-arm64.zip` を展開します。
2. ターミナルで展開先ディレクトリに移動し、隔離属性（quarantine）を解除して起動します。
   ```bash
   cd "/path/to/extracted-folder"
   xattr -rc "utautts.app" tools runtime
   open "utautts.app"
   ```

※配布元が信頼できることを確認した上で実行してください。

---

## Linux（x64）

Linux GUI版の動作には、Qt 6.5以降（Qt Quick, Qt Quick Controls, Qt Multimedia）および日本語フォントが必要です。

Debian / Ubuntu系のパッケージ導入例:
```bash
sudo apt-get update
sudo apt-get install -y fontconfig fonts-noto-cjk \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev \
  qml6-module-qtquick-controls qml6-module-qtmultimedia
```

ZIPを展開後、各実行ファイルに実行権限を付与して起動します:
```bash
chmod +x utautts tools/* runtime/utautts-openjtalk-features runtime/utautts-worldline-bridge
./utautts
```

---

## 更新

更新にはアプリ内アップデーターを使ってください。ZIPを手動で上書きしないでください。更新では `voice/`、`Resamplers/`、`Wavtools/`、`Dependencies/`、`config.ini` と、引き継ぎ可能なユーザーRenderer定義を保持します。Renderer定義は `renderer/<id>/renderer.json` に配置してください。

---

## ボイスバンクの追加

実行ファイルと同じ階層にある `voice/` ディレクトリに、音源フォルダをそのまま配置してください（macOS版は `utautts.app` と同じ階層の `voice/`）。

```text
voice/
  └── 音源名/
        ├── oto.ini
        └── *.wav
```

- 「ファイル」→「音源とプラグイン」→「音源フォルダを開く」からフォルダを直接開けます。
- 音源配置後、アプリを再起動するか「音源を再読込」を実行すると認識されます。
- 配布ZIPファイルをそのまま読み込ませたい場合は、「音源を追加...」からZIPを選択すると `voice/` 直下へ自動展開・登録されます。

※GUI版には「足立レイ ver3.5.0」が同梱されています。利用条件は [同梱音源](voicebank.md) を確認してください。

---

## HTTP Serverの起動

GUI版・Server専用版とも、展開先フォルダで以下のコマンドを実行します（GUIとは独立して起動可能）:

- **Windows**:
  ```powershell
  .\utautts-server.exe --voice-dir ".\voice"
  ```
- **Linux**:
  ```bash
  chmod +x utautts-server runtime/utautts-openjtalk-features runtime/utautts-worldline-bridge
  ./utautts-server --voice-dir ./voice
  ```
- **macOS**:
  ```bash
  xattr -rc "utautts-server" runtime
  ./utautts-server --voice-dir ./voice
  ```

起動後、ブラウザで `http://127.0.0.1:8080/` にアクセスするとWebコンソールUIが開きます。詳細は [UtauTTS Server](server.md) を参照してください。
