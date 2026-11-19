# 開発環境とビルド

コマンドはリポジトリ直下で実行します。

## 必要要件

- Go: 1.27.0 以降
- Qt: Qt 6.5 以降（Qt Quick, Qt Quick Controls, Qt Multimedia, Qt Concurrent）
- CMake & Ninja
- Python: 3.12 以降（同梱音源のZIP展開用）

---

## OS別のビルドコマンド

| 対象OS | 実行環境 | コマンド |
| :--- | :--- | :--- |
| Windows x64 | Windows (PowerShell / CMD) | `.\build.bat win` |
| Linux x64 | Linux (Debian / Ubuntu等) | `./build.sh linux` |
| Linux x64 | Windows (WSL2) | `.\build.bat linux` |
| macOS arm64 | Apple Silicon Mac | `./build.sh macos` |

配布用のZIPは `release/` に出力されます。

参照照合に使うモデル（約1GB）はGitで管理していません。ビルドの前に、Windowsでは `tools/fetch-reference-models.ps1`、Linux・macOSでは `tools/fetch-reference-models.sh` を実行して `models/reference/` へ取得してください。

---

## 開発用サーバーの起動

ソースから直接ローカルAPIサーバーを起動するときは次を実行します。
- Windows: `.\dev.bat`
- Linux / macOS: `./dev.sh`

WORLDとOpen JTalkのライブラリおよび辞書は `runtime/` から読み込みます。初回はWindowsなら `tools/build-native-engines.ps1`、Linux・macOSなら `tools/build-native-engines.sh` を実行して用意してください。

---

## Web版のビルド

Web版はGoのWasmとQtのWasmを別々にビルドしてから組み立てます。

1. emsdkを有効にします。
2. `web/build.ps1`（Linux・macOSは `web/build.sh`）でGoのWasm、JSブリッジ、ONNX Runtime Web、同梱モデルと音源を `web/dist` に出力します。
3. `qt/wasm/build.ps1`（Linux・macOSは `qt/wasm/build.sh`）でQtのWasmをビルドし、`build/web-release` にまとめます（Linux・macOSでは `QT_WASM_DIR` と `QT_HOST_PATH` を指定）。

参照照合のモデルは大きいため、`tools/web-reference/export.py` で作成したフォルダを `UTAUTTS_WEB_REFERENCE` に指定した場合のみコピーします。

---

## テストの実行

```bash
go test ./...
go vet ./...
```
