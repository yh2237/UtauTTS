# リリーステスト

正式リリースではソース上の単体テストだけでなく配布ZIPを展開した状態でも機能を確認します。

## 一括実行

Windows版をWindows上でビルドして全テストを実行する場合は、リポジトリ直下で次を実行します。

```powershell
.\build.bat win
```

この処理はGoの全テスト、GUI・CLI・Serverのビルド、ライセンス収集、ZIP作成、配布物スモークテストを順番に実行します。途中で一つでも失敗すればリリースビルド全体が失敗します。

作成済みのZIPだけを再検査する場合は次を実行します。

```powershell
./tools/test-release-package.ps1
```

Linux版はDebian／Ubuntu上で直接一括実行できます。

```bash
./build.sh linux
```

WindowsからLinux版を検査する場合は、WSL2側で一度セットアップした後、Windows側から次を実行します。

```powershell
.\build.bat linux
```

`build.bat linux`はWSL側のLinuxビルドと同じパッケージ検査まで実行します。WSLの準備方法は[開発環境とビルド](building.md#windowsからlinux-x64をビルドwsl2)を確認してください。

作成済みのLinux ZIPだけを再検査する場合はLinux環境で次を実行します。

```bash
./tools/test-linux-package.sh
```

Linux検査ではZIPを一時ディレクトリへ展開して日本語フォント、共有ライブラリ解決、実行権限、GUI ELFのCOPY relocation／TEXTREL、QtオフスクリーンGUI自己診断、CLI合成、Serverの解析・合成・batch APIを確認します。PipeWire／PulseAudioのセッションがない完全なヘッドレス環境ではGUI自己診断だけを自動的にスキップし、CLIとServerの検査を続行します。GUI自己診断を必須にする場合は`UTAUTTS_REQUIRE_GUI_SELF_TEST=1`を設定してください。

Windowsの標準ビルドは`Full`プロファイルです。作成済みの日本語軽量版を検査する場合は、ビルド時と同じプロファイルを指定します。

```powershell
.\tools\build-release.ps1 -Profile Japanese
.\tools\test-release-package.ps1 -Profile Japanese
```

## 更新経路とリリースメタデータ

リリース前には、`appinfo.json`のversionと更新schemaが前リリースから後退していないことを確認します。v1.2.2から最初の更新を作る場合は、前バージョンだけを渡せば、v1.2.2の旧metadata baselineを検査側が補います。

```powershell
$expected = 'v1.2.3'  # 実際に作成するタグへ置き換える
.\tools\check-release.ps1 `
  -ExpectedVersion $expected `
  -PreviousVersion v1.2.2
```

`build-release.ps1`と`test-release-package.ps1`もこの検査を呼び出します。`go test ./cmd/utautts-updater`には、v1.2.2形式の更新が現行パッケージを導入できること、音源・設定を保持できること、旧Renderer定義を移行できることを確認するテストが含まれます。Qtの配布物self-testでは、初回起動migration schemaの記録とpending update markerの消去も確認します。

GUIの手動確認では、更新通知を有効にした状態で安定版だけが候補になること、開発者モードを有効にした後だけ「プレリリース版も確認する」が表示されること、両方を有効にしたときだけプレリリース版が候補になることを確認します。同じ数値バージョンの安定版とプレリリース版がある場合は安定版を選びます。

## 自動確認する機能

| 対象 | 確認内容 |
| --- | --- |
| Go内部処理 | frontend、音源読込、原音選択、各Renderer、抑揚、WAV、exo、API、更新処理 |
| 配布内容 | 必須ファイル、モデル、Renderer、runtime、ライセンス、不要な開発用ファイルの不在 |
| GUI | QMLロード、Goネイティブ接続、音源・モデル・Renderer列挙 |
| GUI編集基盤 | プロジェクト保存・読込、辞書と設定の保存 |
| GUI音声処理 | 文章解析、抑揚予測、実運用Rendererでの合成、WAV保存 |
| GUI出力 | exo出力、抑揚教師データの途中保存・読込・書出し |
| CLI | `waveform`による最小合成、v8モデルと既定Rendererによる実運用合成、不正数値の拒否 |
| HTTP server | コンソール、health、音源・モデル・Renderer一覧、解析、`waveform`／既定Renderer合成、batch ZIP、音源再読込 |

GUIの検査にはWindowsなら配布された`app/utautts-gui.exe --self-test`、Linuxなら`QT_QPA_PLATFORM=offscreen ./utautts --self-test`を使います。画面や更新確認は開かずテスト専用の一時設定と一時ファイルだけを使うようになっています。
