# 開発環境とビルド

コマンドはリポジトリ直下で実行します。

## 必要要件

- Go: 1.27.0 以降
- Qt: Qt 6.5 以降（Qt Quick, Qt Quick Controls, Qt Multimedia, Qt Concurrent）
- CMake & Ninja
- Python: 3.12 以降（同梱音源のZIPの展開用）

---

## OS別のビルドコマンド

| 対象OS | 実行環境 | コマンド |
| :--- | :--- | :--- |
| Windows x64 | Windows (PowerShell / CMD) | `.\build.bat win` |
| Linux x64 | Linux (Debian / Ubuntu等) | `./build.sh linux` |
| Linux x64 | Windows (WSL2) | `.\build.bat linux` |
| macOS arm64 | Apple Silicon Mac | `./build.sh macos` |

配布用のZIPは `release/` にできます。

参照照合に使うモデル（約0.7GB）はGitで管理していません。ビルドの前に、Windowsでは`tools/fetch-reference-models.ps1`、Linux・macOSでは`tools/fetch-reference-models.sh`を実行して、`models/reference/`に取得してください。

---

## 開発用サーバーの起動

ソースから直接ローカルAPIサーバーを起動するときは、次を実行します。
- Windows: `.\dev.bat`
- Linux / macOS: `./dev.sh`

WORLDとOpen JTalkのライブラリ、Open JTalkの辞書は`runtime/`から読み込みます。初回は、Windowsでは`tools/build-native-engines.ps1`、Linux・macOSでは`tools/build-native-engines.sh`を実行して用意してください。

---

## テストの実行

```bash
go test ./...
go vet ./...
```
