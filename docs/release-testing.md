# リリーステスト手順

## 1. パッケージ作成と一括テスト

次のコマンドで、単体テスト、ビルド、ライセンス収集、ZIP作成、基本動作の確認をまとめて実行します。

- Windows: `.\build.bat win`
- Linux（Debian / Ubuntu / WSL2）: `./build.sh linux`
- macOS（Apple Silicon）: `./build.sh macos`

---

## 2. 生成済みパッケージ単体の検査

ビルド済みのZIPで動作検査だけをやり直すときは次を実行します。

- Windows: `./tools/test-release-package.ps1`
- Linux: `./tools/test-linux-package.sh`
- macOS: `./tools/test-macos-package.sh`

Linuxの検査では一時ディレクトリへのZIP展開、依存ライブラリの解決、実行権限、ELFバイナリのCOPY relocation / TEXTRELの有無、QtオフスクリーンでのGUI自己診断、CLIでの合成、HTTP Server APIの動作を確認します。

---

## 3. バージョン更新互換性テスト

前のバージョンから更新したときに設定や追加音源が消えずに残るか確認します。あらかじめ `internal/appinfo/appinfo.json` のバージョンを前のバージョンと違う値に変更し、配布パッケージを作成しておきます。

```powershell
.\tools\test-update-compatibility.ps1 -PreviousVersion v1.7.1
```
