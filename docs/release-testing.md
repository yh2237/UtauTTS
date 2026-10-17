# リリーステスト手順

## 1. パッケージ作成と一括テスト

次のコマンドで、単体テスト、ビルド、ライセンスの収集、ZIPの作成、基本動作の確認をまとめて行います。

- Windows:
  ```powershell
  .\build.bat win
  ```
- Linux (Debian / Ubuntu / WSL2):
  ```bash
  ./build.sh linux
  ```
- macOS (Apple Silicon):
  ```bash
  ./build.sh macos
  ```

---

## 2. 生成済みパッケージ単体の検査

ビルド済みのZIPで、動作の検査だけをやり直すときは次を実行します。

- Windows:
  ```powershell
  ./tools/test-release-package.ps1
  ```
- Linux:
  ```bash
  ./tools/test-linux-package.sh
  ```
- macOS:
  ```bash
  ./tools/test-macos-package.sh
  ```

Linuxの検査では一時ディレクトリへのZIPの展開、依存ライブラリの解決、実行権限、ELFバイナリのCOPY relocation / TEXTRELの有無、QtのオフスクリーンでのGUI自己診断、CLIでの合成、HTTP Server APIの動作を確かめます。

---

## 3. バージョン更新互換性テスト

前のバージョンから更新したときに、設定や追加した音源が消えずに残るかを確かめます。

```powershell
.\tools\test-update-compatibility.ps1 -PreviousVersion v1.5.1
```
