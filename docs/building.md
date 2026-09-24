# 開発環境とビルド

リポジトリを取得した後に、テスト、各OSのビルド、配布物の検査を行う手順を説明します。

## ビルド方法

| 実行環境 | Windows版 | Linux版 | macOS版 |
| --- | --- | --- | --- |
| Debian／UbuntuなどのLinux | — | `./build.sh linux`（ネイティブ） | — |
| Windows PowerShell／コマンドプロンプト | `./build.bat win` | `./build.bat linux`（WSL2） | — |
| Windows Git Bash | `./build.bat win` | `./build.sh linux`（WSL2） | — |
| Apple Silicon macOS | — | — | `./build.sh macos` |

WindowsからLinux版を作成する場合はWSL2を使います。macOS版はApple Silicon MacまたはGitHub Actionsで作成します。

## 共通の準備とテスト

Go 1.27.0以降が必要です。依存モジュールとリリース用ファイルを取得するため、ビルド時にインターネット接続が必要になる場合があります。

通常のテストはリポジトリ直下で実行します。

```bash
./tools/test.sh
```

WindowsでGoがPATHにある場合は、PowerShellから次を実行できます。

```powershell
go test ./...
go vet ./...
```

## リリース用ファイルとライセンス

リリースビルドの条件と出典は、[ライセンスの適用範囲](../LICENSE-SCOPE.md)、[第三者通知](../THIRD_PARTY_NOTICES.txt)、`THIRD_PARTY_NOTICES-*`、`../licenses/`、各コンポーネントの同梱文書を参照してください。

Go依存の収集対象は[go-license-modules.txt](../tools/go-license-modules.txt)で管理します。依存の追加・削除時は、この一覧と配布物の検査対象を更新します。辞書などのデータ通知は収集スクリプトで個別に指定します。

リリースビルドでは、Go依存のライセンス本文を`licenses/Go/`へ保存します。同じ本文は一度だけ収録します。Open JTalkヘルパーの通知は`runtime/licenses/`へ、辞書の`COPYING`は辞書ディレクトリ内へ保存します。QtのSPDX JSONは監査用に`build/license-audit/Qt/`へ保存し、配布物のQt通知は`licenses/Qt/`へ収録します。

WindowsとmacOSのQt GUIのFFmpeg構成: 利用可能なネイティブバックエンド。外部のQt Multimedia用FFmpegバックエンドは、プラグインとコーデックのフォルダを設定画面で指定します。初回起動時の環境変数: 次の一覧を上から順に確認し、最初に見つかったパスを保存します。

```text
UTAUTTS_FFMPEG_PATH
FFMPEG_PATH
FFMPEG_DIR
FFMPEG_ROOT
```

リリースアーカイブには`Qt-SBOM-MANIFEST.txt`と`FFmpeg-OPTIONAL.txt`を含めます。

WindowsのOpen JTalkヘルパーのランタイムDLL検出元: 公式のVisual C++再頒布用ディレクトリとWindows SDKのUCRT再頒布用ディレクトリ。PyInstallerのPATHは検出対象外。検出できない場合の指定変数: x64用ディレクトリを次の変数へ設定します。

```powershell
$env:UTAUTTS_MSVC_REDIST_DIR = 'C:\path\to\Microsoft.VC143.CRT'
$env:UTAUTTS_UCRT_REDIST_DIR = 'C:\path\to\Windows Kits\10\Redist\ucrt\DLLs\x64'
```

同梱DLLは公式の再頒布用ディレクトリから取得し、開発環境のPATHにあるDLLは配布対象外です。macOSでは同じ名前の環境変数を`export`で設定します。値を特定できないQt GUIビルドは失敗させます。

## Linux x64

Linuxネイティブ環境とWSL環境では、同じセットアップスクリプトを使います。実行するLinux環境ごとに一度だけ、リポジトリ直下で実行してください。

```bash
./tools/setup-linux.sh
```

このスクリプトは、Qt 6.5以降（Qt Quick、Qt Multimedia、Qt Concurrent）、CMake、Ninja、`readelf`、Python仮想環境、pyopenjtalk 0.4.1、PyInstaller 6.16.0などを準備し、Go 1.27.0以降を確認します。APT処理を省略する場合は`UTAUTTS_SKIP_APT=1`を指定します。

```bash
UTAUTTS_SKIP_APT=1 ./tools/setup-linux.sh
```

Linux版のGUIとServerをビルドし、ZIPの基本動作検査まで行います。

```bash
./build.sh linux
```

ビルド中とZIP展開後にLinux GUIのELFを検査し、COPY relocationとTEXTRELがあれば失敗します。GUIを起動できない環境でも、この検査とCLI／Serverの検査は実行されます。`bash tools/build-linux.sh`でも同じ処理を実行できます。出力は`release/`へ作成します。

開発用Serverは次で起動します。

```bash
./dev.sh
```

## Windows x64

次の開発環境が必要です。

- Go 1.27.0以降
- Qt 6.5以降（Qt Quick、Qt Multimedia、Qt Concurrent）
- CMakeとNinja
- MSYS2 Clang
- Python 3.12 x64（Open JTalkヘルパーのビルド用）

Qt SDKを`.qt/<version>/mingw_64`へ置くと自動検出します。別の場所に置く場合は`QT_ROOT`を設定します。MSYS2やQt Toolsの場所が標準と異なる場合は`MSYS2_ROOT`、`QT_MINGW_ROOT`、`QT_TOOLS_ROOT`を設定します。

```powershell
.\build.bat win
```

GUI版とServer版のZIPが`release/`へ作成され、配布物の基本動作検査まで実行されます。既定の`Full`プロファイルではDiffSingerのruntimeも作成します。日本語向けの軽量版は次のコマンドで作成します。

```powershell
.\tools\build-release.ps1 -Profile Japanese
```

`Japanese`では`diffsinger` Rendererと対応runtimeを除外します。開発用Serverは`.\dev.bat`で起動します。

## macOS arm64

macOS版はApple Silicon（arm64）向けです。Go、Python 3.12以降、CMake、Ninja、Qt 6.5以降、`zip`、`unzip`、`curl`、`shasum`を用意してください。Qtを標準外の場所に置く場合は`QT_ROOT`を設定します。

```bash
QT_ROOT=/path/to/Qt ./build.sh macos
```

`tools/build-macos.sh`を直接実行しても同じ処理になります。GUI版とServer版のZIPを`release/`へ作成し、配布物の基本動作検査まで実行します。Mac実機を使えない場合はGitHub ActionsのmacOSワークフローを利用できます。

## WindowsからLinux x64を作成する場合

WSL2にDebianまたはUbuntuを用意し、WSLターミナルで次を一度実行します。

```bash
cd /mnt/c/path/to/UtauTTS
./tools/setup-linux.sh
```

その後、WindowsのPowerShellまたはコマンドプロンプトから実行します。

```powershell
.\build.bat linux
```

Git Bashからは`./build.sh linux`でも実行できます。既定のWSLディストリビューション以外を使う場合は、Windows側で`UTAUTTS_WSL_DISTRO`を設定します。

```powershell
$env:UTAUTTS_WSL_DISTRO = 'Debian'
.\build.bat linux
```

Windows版とLinux版は`.\build.bat both`で連続してビルドできます。WSL側の`.env`にはWSLから見えるLinuxパスを書きます。WSLのLinux版ビルドではGoキャッシュをプロジェクト内の`build/go-mod-cache`に置きます。

## 環境変数

通常は`.env`なしで動きます。雛形は[`.env.example`](../.env.example)を参照してください。`.env`は環境ごとの設定を書くファイルで、Git管理対象外です。

LinuxでQtを標準外の場所に置く場合、または開発用Serverの音源を変更する場合だけ、次の設定を`.env`へ記述します。

```text
QT_ROOT=/path/to/Qt
UTAUTTS_VOICE_DIR=/path/to/voicebank
```

GoやPythonを一時的に差し替える場合は、コマンド単位で指定できます。

```bash
GO_BIN=/path/to/go PYTHON=/path/to/python ./build.sh linux
```

## `tools/`の役割

`tools/`には、開発、ビルド、リリース、配布物検査、モデル再生成を補助するスクリプトを置きます。配布物の検査手順は[リリーステスト](release-testing.md)にまとめています。
