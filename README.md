# UtauTTS

UTAUボイスバンクの原音接続に学習ベースのイントネーション調整を加えた日本語TTS

> ボイスバンクを使う前に、各音源の利用規約を確認してください。UtauTTSと作者は、ボイスバンクの利用で生じた問題について責任を負いません。

## インストール

[GitHub Releases](https://github.com/yh2237/UtauTTS/releases)から環境と用途に合うZIPをダウンロードします。

| パッケージ | 用途 |
| --- | --- |
| `UtauTTS-win-x64.zip` | Windows x64向けGUIとCLI |
| `UtauTTS-linux-x64.zip` | Linux x64向けGUIとCLI |
| `UtauTTS-Server-win-x64.zip` | Windows x64向けHTTP Server |
| `UtauTTS-Server-linux-x64.zip` | Linux x64向けHTTP Server |

Windows版はZIPを展開して`utautts.exe`を実行します。

Linux版はQt 6.5以降、Qt Quick、Qt Multimedia、日本語フォントが必要です。ZIPを展開し実行権限を付けて起動します。

```bash
chmod +x utautts tools/* runtime/utautts-openjtalk-features runtime/utautts-worldline-bridge
./utautts
```

Debian／Ubuntu向けのパッケージ名などは[インストール](docs/installation.md)にあります。

GUI版には「足立レイ ver3.5.0」を同梱しています。利用条件は[同梱ボイスバンク](docs/voicebank.md)と音源内の文書を確認してください。

## GUIで音声を作る

次の順で操作します。

1. 文章欄へ文を入力する
2. `Ctrl+Enter`または再生を押す
3. 下のグラフでイントネーションとモーラ長を確認する
4. 必要に応じて点や境界線を動かす
5. 「ファイル」→「WAVを保存...」で保存する

文章は追加、削除、並べ替えできます。「WAVをすべて保存...」では、文章が入っているカードをまとめて合成します。

### ボイスバンクを追加する

実行ファイルと同じ階層にある`voice`へ音源のフォルダを置きます。

voiceディレクトリは「ファイル」→「音源フォルダを開く」から開けます。

```text
voice/
  音源名/
    oto.ini
    *.wav
```

以下でも可

```text
voice/
  音源名/
    音源名/
      oto.ini
      *.wav
```

配置後にUtauTTSを再起動するか「ファイル」→「音源を再読込」を選択することで再読み込みされます。

### 文章ごとの設定

右側の設定で選択中の文章に使う音源や合成方法を変更できます。

| 項目 | 内容 |
| --- | --- |
| 音源 | 使用するボイスバンク |
| 原音形式 | CV、VCV、CVVCの選び方。通常は`自動` |
| 抑揚モデル | 自動イントネーションやモーラ長の予測に使うモデル |
| Renderer | 原音の長さと高さを変え、接続してWAVにする方式 |
| 音高 | `prefix.map`から選ぶ音階 |
| 抑揚 | 自動イントネーションの強さ |
| モーラ長 | 自動値がない場合に使う基本長 |
| 休止長 | 句読点などの休止時間 |
| 文頭の長さ | 最初の原音に確保する先行発声。通常は`自動` |

文頭が欠ける音源では「文頭の長さ」を長くし、余計なノイズを拾う音源では短くしてください。新規作成したカードに使う既定値は「設定」→「設定...」から変更できます。初期状態では抑揚2、モーラ長120 ms、休止長180 ms、抑揚モデル`frame-intonation-v8`、Renderer`openutau-worldline-r-faithful`です。

### イントネーションと長さを直す

テキストの解析が終わるとモデルが予測した値がグラフへ表示されます。

- 点を上下へ動かすとそのモーラのピッチが変わります。
- 縦線を左右へ動かすとモーラの長さが変わります。
- `Shift`を押しながら縦線を動かすと後ろの境界もまとめて移動します。
- `Ctrl+Z`と`Ctrl+Y`でピッチとモーラ長のUndo／Redoを行います。

### 保存と連携

`.utautts`プロジェクトには文章、カードの順序、音源やRendererなどの合成設定、手動編集した値が保存されます。

設定を有効にするとWAVと同名のファイルも書き出せます。

- `.txt`: 入力した文章。UTF-8またはShift_JIS
- `.lab`: HTK形式の推定音素境界。PSDToolKitなどのリップシンク用
- `.exo`: 各文章のWAVを1レイヤーへ並べたAviUtl拡張編集用ファイル

exo出力後に表示される領域をAviUtlの拡張編集へドラッグすると作成したWAVを読み込めます。

### Renderer

| Renderer | 特徴 |
| --- | --- |
| `openutau-worldline-r-faithful` | 既定。原音の音響特徴を時間軸へ配置し、フレーズ全体を再合成 |
| `openutau-classic-worldline-faithful` | OpenUTAU Classicと同じWorldline処理と5点包絡線で原音ごとに接続 |
| `waveform` | Go内で原音波形を伸縮して接続。原音の明瞭度を確認しやすい |
| `openutau-classic-worldline-faithful-gpu` | Classic faithfulのCUDA版。対応するWindows配布物だけに同梱 |

### 抑揚モデル

| モデル | 内容 |
| --- | --- |
| `frame-intonation-v8` | Open JTalkのアクセント特徴からフレーム単位のイントネーションを予測 |
| `prosody-multitask-v1` | v8系のイントネーションに加えてモーラ長も予測 |

モデルやRendererはGUI、CLI、Serverで共通です。追加方法は[モデル／Rendererプラグイン](docs/plugins.md)、GUIで集めた手動調整から個人用モデルを作る手順は[手動調整から抑揚モデルを作る](docs/prosody-model-training.md)にあります。

## CLI

CLIはGUI版の`tools/utautts-cli.exe`または`tools/utautts-cli`に入っています。次は同梱音源へ自動イントネーションを適用する例です。

```powershell
.\UtauTTS\tools\utautts-cli.exe `
  --voicebank ".\UtauTTS\voice\足立レイver3.5.0" `
  --text "こんにちは、今日はいい天気です。" `
  --renderer openutau-worldline-r-faithful `
  --prosody frame-intonation-v8 `
  --prosody-pitch-only `
  --apply-pitch `
  --out ".\out.wav"
```

読みを直接渡す`--kana`、ユーザー辞書、モーラごとの長さ、TXT／LAB同時保存、合成計画を書き出す`--plan-out`などもあります。全オプションは[コマンドライン](docs/cli.md)を参照してください。

## HTTP Server

Server版を起動すると`http://127.0.0.1:8080/`でコンソールUIを使えます。

```powershell
.\UtauTTS-Server\utautts-server.exe --voice-dir ".\UtauTTS-Server\voice"
```

文章解析、ユーザー辞書、音源・モデル・Rendererの一覧、WAV／LAB、一括ZIPへのTXT／LAB同梱を`/api/*`から利用できます。APIのJSON形式と入力制限は[UtauTTS Server](docs/server.md)にあります。

## うまく動かないとき

[トラブルシューティング](docs/troubleshooting.md)をご覧ください。

解決しない場合はIssueを送るか[@2237yh](https://x.com/2237yh)に直接DMを送ってください。

## 詳細ドキュメント

- [インストール](docs/installation.md)
- [GUIの使い方](docs/gui.md)／[設定](docs/settings.md)
- [コマンドライン](docs/cli.md)／[UtauTTS Server](docs/server.md)
- [UtauTTSはどうやって音声を作るのか](docs/how-utautts-speaks.md)
- [モデル／Rendererプラグイン](docs/plugins.md)
- [開発環境とビルド](docs/building.md)／[技術設計ガイド](docs/technical-design.md)
- [ドキュメント一覧](docs/README.md)

## 謝辞

- Testing by [アアアアアアア（@a7_riri）](https://x.com/a7_riri)

## ライセンス

UtauTTSのソースコードは[MIT License](./LICENSE)です。`models/`の学習済みモデルはCC BY-SA 4.0です。同梱ボイスバンク、学習済みモデル、OpenUtau由来ファイルなどには個別の利用条件があります。配布物の[THIRD_PARTY_NOTICES.txt](./THIRD_PARTY_NOTICES.txt)、`licenses/`、各同梱文書を確認してください。
