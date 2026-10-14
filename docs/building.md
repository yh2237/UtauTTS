# 開発環境とビルド

リポジトリ直下でビルドおよびテストを実行する手順です。

## 必要要件

- **Go**: 1.27.0 以降
- **Qt**: Qt 6.5 以降（Qt Quick, Qt Quick Controls, Qt Multimedia, Qt Concurrent）
- **CMake & Ninja**
- **Python**: 3.12 以降（Open JTalkヘルパーのビルド用）

---

## OS別のビルドコマンド

| 対象OS | 実行環境 | コマンド |
| :--- | :--- | :--- |
| **Windows x64** | Windows (PowerShell / CMD) | `.\build.bat win` |
| **Linux x64** | Linux (Debian / Ubuntu等) | `./build.sh linux` |
| **Linux x64** | Windows (WSL2) | `.\build.bat linux` |
| **macOS arm64** | Apple Silicon Mac | `./build.sh macos` |

ビルドが完了すると、`release/` ディレクトリに配布用ZIPアーカイブが生成されます。

---

## 開発用サーバーの起動

開発中にローカルAPIサーバーを直接起動する場合:
- **Windows**: `.\dev.bat`
- **Linux / macOS**: `./dev.sh`

---

## テストの実行

```bash
# 全テストと静的解析の実行
go test ./...
go vet ./...
```
