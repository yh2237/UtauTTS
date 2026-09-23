# コマンドライン（CLI）

`utautts-cli`はボイスバンク、モデル、Rendererを指定してWAVを作るコマンドライン合成ツールです。GUIやHTTP Serverと同じ合成処理を使います。

配布物ではWindowsの`tools/utautts-cli.exe`、Linuxの`tools/utautts-cli`にあります。開発時は`go run ./cmd/utautts-cli`でも実行できます。

## 必須の引数

- `--voicebank <dir>`: UTAUボイスバンクのディレクトリ
- `--out <path>`: 出力WAVのパス
- `--text <文>`または`--reading <読み>`: 合成する文章または読み

`--text`は選択した言語と発音形式で読みを生成します。`--reading`は、かな、ARPAbet、Pinyinなどの読みを直接指定します。`--kana`は互換用の別名です。日本語以外を使う場合は`--language`も指定してください。

## 基本例

同梱音源と原音の明瞭度を確認しやすい`waveform` Rendererを使う最小例です。

```powershell
.\UtauTTS\tools\utautts-cli.exe `
  --voicebank ".\UtauTTS\voice\足立レイver3.5.0" `
  --text "あらゆる現実をすべて自分のほうへねじ曲げたのだ。" `
  --renderer waveform `
  --out ".\out.wav"
```

学習イントネーションを適用する例です。モデルIDとframe pitch対応Rendererを指定します。配布物内のOpenJTalk frontendが実行時に読みとアクセント特徴を生成します。

```powershell
.\UtauTTS\tools\utautts-cli.exe `
  --voicebank ".\UtauTTS\voice\足立レイver3.5.0" `
  --text "あらゆる現実をすべて自分のほうへねじ曲げたのだ。" `
  --renderer utautts-world-phrase `
  --prosody frame-intonation-v9-t `
  --prosody-pitch-only `
  --apply-pitch `
  --out ".\out.wav"
```

`--prosody`へモデルIDを指定すると別の抑揚モデルを使えます。`--plan-out`を指定すると原音の配置とタイミングをJSONへ保存できます。

英語では`--prosody english-intonation-v1`を指定します。英語用モデルはOpen JTalkを使わず、`utautts-world-phrase`のようなframe pitch対応Rendererで強勢と句末境界を適用します。

GUIと同じユーザー辞書は、次のJSONを`--dictionary dictionary.json`で渡します。

```json
[
  {"surface": "v8", "reading": "ぶいはち"},
  {"surface": "UtauTTS", "reading": "うたうてぃーてぃーえす"}
]
```

`--write-text`または`--write-lab`を指定すると、WAVと同じ場所へ同名の`.txt`または`.lab`を保存します。

## オプション

`--speech-timing`は[発話タイミング補正](speech-quality-experiment.md)を有効にします。CLIでは初期状態で無効です。

| オプション | 既定値 | 説明 |
|---|---|---|
| `--version` | | アプリケーションのバージョンを表示して終了 |
| `--voicebank <dir>` | | ボイスバンクのディレクトリ（必須） |
| `--text <文>` | | 合成する文章 |
| `--reading <読み>` | | かな、ARPAbet、またはPinyinを直接指定 |
| `--kana <読み>` | | `--reading`と同じ入力を受け付ける互換用の別名 |
| `--language <id>` | `ja` | 言語。`ja`、`en`、`zh` |
| `--phonemizer <id>` | 言語から自動選択 | phonemizer。`ja-kana`、`en-arpasing`、`en-delta`、`en-vccv`、`en-cv`、`zh-cvvc` |
| `--tone` | `C4` | `prefix.map` 使用時に使う音階 |
| `--color <name>` | | `character.yaml`で定義された音源タイプ／サブバンク |
| `--out <path>` | | 出力WAVのパス（必須） |
| `--plan-out <path>` | | 合成計画JSONを保存するパス |
| `--ustx-out <path>` | | 合成パラメータをOpenUtauのUSTXプロジェクトへ保存するパス |
| `--dictionary <path>` | | 表記と読みを定義したユーザー辞書JSON |
| `--mora-ms` | `140` | 基本モーラ長（ms） |
| `--pause-ms` | `180` | 句読点の休止長（ms） |
| `--mora-durations <path>` | | モーラごとの長さを配列または`mora_durations_ms`で持つJSON |
| `--leading-preutterance-ms` | `0` | 文頭に確保する先行発声（ms）。0では`oto.ini`から自動決定 |
| `--release-ms` | `20` | ユニット末尾のリリース包絡線（ms） |
| `--prosody <id>` | | 抑揚モデルのplugin ID |
| `--prosody-pitch-only` | `false` | 学習ピッチのみ適用し、モーラ長・音量は固定値を使う |
| `--manual-pitch <path>` | | 手動ピッチ編集JSON（[manual-pitch.md](manual-pitch.md)） |
| `--prosody-features <path>` | | ケース別のモーラ単位アクセント特徴JSON |
| `--prosody-feature-case <id>` | | `--prosody-features` 内のケースID |
| `--pitch-contours <path>` | | ケース別ピッチ係数JSON（計画へ記録。波形処理には `--apply-pitch` が必要） |
| `--pitch-case <id>` | | `--pitch-contours` 内のケースID |
| `--apply-pitch` | `true` | 波形のピッチ再サンプリング |
| `--intonation-strength` | `2` | 音源ピッチ安定化と句曲線の強さ（0〜4） |
| `--context-duration` | `true` | 日本語モーラ長の文脈連動（C1）を有効にする |
| `--context-duration-strength` | `1` | 文脈連動の強度（0〜2）。0は既定1.0として扱う |
| `--boundary-tone` | `true` | 日本語の句末境界音調（C2）を有効にする |
| `--boundary-tone-strength` | `1` | 境界音調の強度（0〜2）。0は既定1.0として扱う |
| `--stretch-adapt` | `true` | 音源実測に基づき日本語モーラの過度な伸縮を有界にする（C3a）。長いモーラ長（例: 200ms以上）のときのみ有効。既定の短い設定では無効（解析コスト回避） |
| `--stretch-adapt-strength` | `1` | 伸縮補正の強度（0〜2）。0は既定1.0として扱う |
| `--renderer <id>` | 既定Renderer | Renderer ID（省略時は設定された優先度が最大のもの。未知の明示IDはエラー） |
| `--resampler <id>` | 自動選択 | Classic UTAUで使う`Resamplers/`からの相対ID |
| `--wavtool <id>` | `builtin` | Classic UTAUで使う`Wavtools/`からの相対ID |
| `--resampler-expressions <path>` | | unit単位のresampler設定JSON |
| `--worldline-bridge <path>` | | `utautts-worldline-bridge` 実行ファイル |
| `--boundary-bridge-ms` | `0` | 位相を揃えた波形接続補修の最大幅（0で無効、開発者・評価用） |
| `--boundary-bridge-threshold` | `0` | 標準の接続評価がこの値以下のとき接続補修を適用（開発者・評価用） |
| `--alias-policy` | `auto` | 音源適応モード。`auto`はVC/VCV収録比から自動選択、`cvvc-enhanced`はCVVC優先・sequential timing・VC音量35%。詳細指定として`vcv-prefer`、`cvvc-prefer`、`cv-only`も利用可能 |
| `--cvvc-timing` | `sequential` | CVVC遷移の配置方式。現在は`sequential`のみ |
| `--cvvc-transition-gain` | `1` | CVVC遷移ユニットの音量（0〜1） |
| `--diffsinger-steps` | `0`（既定値） | DiffSingerの拡散ステップ数。0で既定値 |
| `--diffsinger-duration-mix` | `0`（既定値） | DiffSingerの長さ予測の混合率（0〜1）。0で既定値 |
| `--diffsinger-pitch-mix` | `0`（既定値） | DiffSingerのピッチ予測の混合率（0〜1）。0で既定値 |
| `--diffsinger-expr` | `0`（既定値） | DiffSingerの表現力（0〜2）。0で既定値1.0。下げると話声寄り |
| `--cvvc-pre-boundary-fade` | `false` | 後続CVの子音より前でCVVC遷移をフェードアウト |
| `--renderer-dir <dir>` | | Rendererプラグインを探すディレクトリ（繰り返し指定可） |
| `--model-dir <dir>` | | モデルJSONを探すディレクトリ（繰り返し指定可） |
| `--openjtalk-features <path>` | runtime | Open JTalk feature helper（自動検出を上書き） |
| `--openjtalk-dictionary <path>` | runtime | Open JTalk辞書ディレクトリ（自動検出を上書き） |
| `--write-text` | `false` | WAVと同名のTXTを書き出す |
| `--write-lab` | `false` | WAVと同名のHTK形式LABを書き出す |
| `--text-encoding` | `utf-8` | TXTの文字コード（`utf-8`または`shift_jis`） |

## モデルとRendererの指定

`--prosody`と`--renderer`にはファイルパスではなくIDを指定します。Classic UTAUのツールも絶対パスではなく、`Resamplers/`または`Wavtools/`からの相対IDを指定します。一覧は[モデル／Rendererプラグイン](plugins.md)を参照してください。

モデルやRendererは実行ファイルの隣にある`models/`と`renderer/`から自動検出します。別の検索先を追加する場合は`--model-dir`または`--renderer-dir`で指定します。明示したRenderer定義は同梱定義より優先されます。

`--renderer`を省略した場合はカタログの`default_priority`が最大のRendererを使います。存在しないIDを明示した場合はエラーになります。指定したRendererの必要なファイルが不足している場合もエラーになります。

`--resampler-expressions`のJSONは[Classic UTAU互換仕様](plugins.md#classic-utau互換仕様)を参照してください。

`--apply-pitch`と`--intonation-strength`によるピッチ加工は、音源によって声質や明瞭度に影響する場合があります。

## 出力

成功するとWAVを書き出して次の行を表示します。`--write-text`と`--write-lab`の内容はGUIから保存した場合と同じです。

```
wrote out.wav (4.81s, 44100 Hz, 34 units)
```

合成の失敗や引数エラーは終了コード1を返します。原音の明瞭度を確認するなら`--renderer waveform`を使います。

## USTXへの一括変換

保存済みの`.utautts`プロジェクトは、配布物の`tools/utautts-ustx.exe`または`tools/utautts-ustx`でOpenUtauのUSTXへ変換できます。開発時は`go run ./cmd/tools/utautts-ustx`でも実行できます。

```console
utautts-ustx project.utautts project.ustx
```

出力パスを省略すると、元のプロジェクトと同じ場所へ`<名前>.ustx`として保存します。GUIの「USTXとして書き出す」では、解析前のカードも書き出すときに解析します。
