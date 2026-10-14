# リリーステスト手順

リリース前に実施する単体テスト、パッケージビルド、および配布ZIPの自己診断テストの手順です。

## 1. パッケージ作成と一括テスト

各プラットフォームで以下のコマンドを実行すると、単体テスト、バイナリビルド、ライセンス収集、ZIP生成、および基本動作確認が一括で実行されます。

- **Windows**:
  ```powershell
  .\build.bat win
  ```
- **Linux (Debian / Ubuntu / WSL2)**:
  ```bash
  ./build.sh linux
  ```
- **macOS (Apple Silicon)**:
  ```bash
  ./build.sh macos
  ```

---

## 2. 生成済みパッケージ単体の検査

既にビルドされたZIPファイルに対して動作検査のみを再実行する場合:

- **Windows**:
  ```powershell
  ./tools/test-release-package.ps1
  ```
- **Linux**:
  ```bash
  ./tools/test-linux-package.sh
  ```
- **macOS**:
  ```bash
  ./tools/test-macos-package.sh
  ```

Linux検査では、一時ディレクトリへのZIP展開、依存ライブラリの解決状態、実行権限、ELFバイナリのCOPY relocation / TEXTRELの有無、QtオフスクリーンGUI自己診断、CLI合成、およびHTTP Server APIの動作が検証されます。

---

## 3. バージョン更新互換性テスト

以前のバージョンからの更新・移行が正常に行われるか確認します（設定や追加音源が消去されず維持されることを検証）。

```powershell
.\tools\test-update-compatibility.ps1 -PreviousVersion v1.5.1
```
