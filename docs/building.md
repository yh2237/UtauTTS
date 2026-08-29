# 開発環境とビルド

## ビルド方法の使い分け

Linux版はLinux上で直接ビルドできます。Windows上でLinux版を作る場合はWSL2を使います。

| 実行環境 | Windows版 | Linux版 |
| --- | --- | --- |
| Debian／UbuntuなどのLinux | — | `./build.sh linux`（ネイティブ） |
| Windows PowerShell／コマンドプロンプト | .\build.bat win | .\build.bat linux（WSL2） |
| Windows Git Bash | `./build.bat win` | `./build.sh linux`（WSL2） |

## 共通

UtauTTSのGoコードはGo 1.27.0を使用します。依存モジュールの取得とリリース用ファイルの取得にはインターネット接続が必要です。

通常のテストはリポジトリ直下で実行します。

```bash
./tools/test.sh
```

WindowsでGoがPATHにある場合は、PowerShellから次を実行できます。

```powershell
go test ./...
go vet ./...
```

リリースビルドで使用する依存物の条件は[THIRD_PARTY_NOTICES.txt](../THIRD_PARTY_NOTICES.txt)と`licenses/`を確認してください。

## Linux x64（Linuxネイティブ／WSL共通）

Linuxネイティブ環境とWSL環境では同じLinuxセットアップスクリプトを使います。実行するLinux環境ごとに一度だけリポジトリ直下で実行してください。

```bash
./tools/setup-linux.sh
```

このスクリプトは次を行います。

- Qt 6.5以降（Qt Quick、Qt Multimedia、Qt Concurrent）とCMake／NinjaなどのAPTパッケージを導入
- Python仮想環境`.venv`を作成し、`pyopenjtalk==0.4.1`と`pyinstaller==6.16.0`を導入
- Go 1.27.0以上を確認し、必要なら公式Linux x64アーカイブをユーザー領域へ導入
- Go、Python、CMake、NinjaをPATHまたは標準のセットアップ先から自動検出

APTパッケージがすでに揃っている環境や、パッケージを導入できないコンテナでは、次のようにAPT処理を省略できます。

```bash
UTAUTTS_SKIP_APT=1 ./tools/setup-linux.sh
```

Linux版のGUIとServerをビルドし、ZIPのスモークテストまで実行します。

```bash
./build.sh linux
```

直接実行する場合は`bash tools/build-linux.sh`でも同じです。出力は`release/`に作成されます。

開発サーバーを起動する場合は次を使います。

```bash
./dev.sh
```

## Windows x64（ネイティブビルド）

次の開発環境が必要です。

- Go 1.27.0以上
- Qt 6.5以降（Qt Quick、Qt Multimedia、Qt Concurrent）
- CMakeとNinja
- MSYS2 Clang
- Python 3.12 x64（Open JTalkヘルパーのビルド用）

Qt SDKを`.qt/<version>/mingw_64`へ置くと自動検出します。別の場所に置く場合はcompiler kitのパスを`QT_ROOT`に設定します。MSYS2やQt Toolsの場所が標準と異なる場合は`MSYS2_ROOT`、`QT_MINGW_ROOT`、`QT_TOOLS_ROOT`を設定できます。

```powershell
.\build.bat win
```

GUI版とServer版のZIPが`release/`へ作成され、そのまま配布物スモークテストまで実行されます。

開発サーバーは次で起動します。

```powershell
.\dev.bat
```

## WindowsからLinux x64をビルド（WSL2）

Windows上でLinux版を作成する場合は、WSL2にDebianまたはUbuntuを用意してください。`build.bat linux`はリポジトリのWindowsパスをWSLパスへ変換し、WSL側の`tools/build-linux.sh`を実行します。

まずWSLターミナルで、Windows側のリポジトリを開いてLinux依存環境を構築します。

```bash
cd /mnt/c/path/to/UtauTTS
./tools/setup-linux.sh
```

その後、WindowsのPowerShellまたはコマンドプロンプトから実行します。

```powershell
.\build.bat linux
```

Git Bashからは次でも同じWSLビルドを実行できます。

```bash
./build.sh linux
```

既定のWSLディストリビューション以外を使う場合だけ、Windows側で`UTAUTTS_WSL_DISTRO`を設定します。

```powershell
$env:UTAUTTS_WSL_DISTRO = 'Debian'
.\build.bat linux
```

Windows版とLinux版を続けて作成する場合は、Windows側で次を実行します。

```powershell
.\build.bat both
```

WSL側の`.env`はLinuxシェルから読み込まれます。Windows用のパスではなく、WSLから見えるLinuxパスを指定してください。

## 環境変数

通常は`.env`を作成する必要はありません。雛形は[`.env.example`](../.env.example)です。`.env`は環境ごとのローカル設定なので、Gitにはコミットしません。

LinuxでQtを標準外の場所に置いた場合、または開発サーバーの音源を変更する場合だけ、次の設定を`.env`に記述します。

```text
# Linux Qtを標準外の場所に置いた場合だけ
QT_ROOT=/path/to/Qt

# 開発サーバーで使う音源を変更する場合だけ
UTAUTTS_VOICE_DIR=/path/to/voicebank
```

GoやPythonを一時的に差し替える場合は、`.env`へ追加せずコマンド単位で指定できます。

```bash
GO_BIN=/path/to/go PYTHON=/path/to/python ./build.sh linux
```

WindowsのPowerShellスクリプトでは、必要に応じて`PYTHON`、`QT_ROOT`、`MSYS2_ROOT`、`QT_MINGW_ROOT`、`QT_TOOLS_ROOT`を環境変数として設定できます。

`WINDOWS_USERNAME`は使用しません。WSLのLinux版ビルドはWindows側ユーザーのGoキャッシュを参照せず、プロジェクト内の`build/go-mod-cache`を使います。

ビルド後に行う配布物の検査は[リリーステスト](release-testing.md)にまとめています。
