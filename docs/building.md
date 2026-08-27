# 開発環境とビルド

## 共通

UtauTTSのGoコードはGo 1.27を使用します。通常のテストはリポジトリ直下で実行します。

```powershell
go test ./...
go vet ./...
```

リリースビルドではOpenUtau由来の依存ファイルを取得してSHA-256を検証するためインターネット接続が必要です。依存物の条件は[THIRD_PARTY_NOTICES.txt](../THIRD_PARTY_NOTICES.txt)と`licenses/`を確認してください。

## Windows x64

次の開発環境が必要です。

- Go 1.27
- Qt 6.5以降（Qt Quick、Qt Multimedia、Qt Concurrent）
- CMakeとNinja
- MSYS2 Clang
- .NET 8 SDK
- Python 3.12 x64

Qt SDKを`.qt/<version>/mingw_64`へ置くと自動検出します。別の場所に置く場合はcompiler kitのパスを`QT_ROOT`に設定します。

```powershell
.\build.bat win
```

GUI版とServer版のZIPが`release/`へ作成されて、そのまま配布物スモークテストまで実行されます。

## Linux x64

Windowsからビルドする場合はWSL2のDebian／Ubuntu環境を使います。WSL側へ次のパッケージを導入してください。

```bash
sudo apt-get update
sudo apt-get install -y build-essential cmake ninja-build pkg-config unzip zip curl wget ca-certificates \
  qt6-base-dev qt6-declarative-dev qt6-multimedia-dev qt6-tools-dev qt6-l10n-tools \
  qml6-module-qtquick-controls qml6-module-qtmultimedia fontconfig fonts-noto-cjk \
  python3 python3-pip python3-venv
```

Goを`/usr/local/go`、.NET 8 SDKを`/opt/dotnet`へ置くとビルドスクリプトがPATHへ追加します。Open JTalk frontendを作るためのPython環境も用意します。

```bash
python3 -m venv /opt/utautts-py
/opt/utautts-py/bin/pip install pyopenjtalk
```

ビルドはWSLのデフォルトユーザー（root以外）を使いWindows側のリポジトリ直下から実行します。

```powershell
.\build.bat linux
```

Windows版とLinux版を続けて作成する場合は次を実行します。

```powershell
.\build.bat both
```

ビルド後に行う検査は[リリーステスト](release-testing.md)にまとめています。
