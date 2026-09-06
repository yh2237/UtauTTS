# コマンドライン（CLI）

`utautts-cli`はボイスバンク・モデル・Rendererを指定して一つのWAVを作るコマンドライン合成ツールです。GUIやHTTPサーバーと同じ合成処理を使います。

配布物ではWindowsの`tools/utautts-cli.exe`、Linuxの`tools/utautts-cli`にあります。開発時は`go run ./cmd/utautts-cli`でも実行できます。

## 必須の引数

- `--voicebank <dir>`: UTAUボイスバンクのディレクトリ
- `--out <path>`: 出力WAVのパス
- `--text <文>`または`--reading <読み>`: 合成する文章または読み

`--text`は選択した言語とphonemizerで発音を生成します。`--reading`は読み・ARPAbet・Pinyinを直接指定します。`--kana`は`--reading`の旧名です。日本語以外を使う場合は`--language`も指定してください。

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
  --prosody frame-intonation-v8 `
  --prosody-pitch-only `
  --apply-pitch `
  --out ".\out.wav"
```

モーラ長も予測する場合は`--prosody prosody-multitask-v1`を使います。`--plan-out`を指定すると原音の配置とタイミングをJSONへ保存できます。

GUIと同じユーザー辞書は、次のJSONを`--dictionary dictionary.json`で渡します。

```json
[
  {"surface": "v8", "reading": "ぶいはち"},
  {"surface": "UtauTTS", "reading": "うたうてぃーてぃーえす"}
]
```

`--write-text`または`--write-lab`を指定すると、WAVと同じ場所へ同名の`.txt`または`.lab`を保存します。

## オプション

| オプション | 既定値 | 説明 |
|---|---|---|
| `--version` | | アプリケーションのバージョンを表示して終了 |
| `--voicebank <dir>` | | ボイスバンクのディレクトリ（必須） |
| `--oto <dir>` | | `--voicebank` の旧名（deprecated） |
| `--text <文>` | | 合成する文章 |
| `--reading <読み>` | | かな、ARPAbet、またはPinyinを直接指定 |
| `--kana <読み>` | | `--reading`の旧名 |
| `--language <id>` | `ja` | 言語。`ja`、`en`、`zh` |
| `--phonemizer <id>` | 言語から自動選択 | phonemizer。`ja-kana`、`en-arpasing`、`en-delta`、`en-vccv`、`zh-cvvc` |
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
| `--release-ms` | `20` | ユニット末尾のrelease envelope（ms） |
| `--prosody <id>` | | 抑揚モデルのplugin ID |
| `--prosody-pitch-only` | `false` | 学習ピッチのみ適用し、モーラ長・音量は固定値を使う |
| `--manual-pitch <path>` | | 手動ピッチ編集JSON（[manual-pitch.md](manual-pitch.md)） |
| `--prosody-features <path>` | | ケース別のモーラ単位アクセント特徴JSON |
| `--prosody-feature-case <id>` | | `--prosody-features` 内のケースID |
| `--pitch-contours <path>` | | ケース別ピッチ係数JSON（計画へ記録。波形処理には `--apply-pitch` が必要） |
| `--pitch-case <id>` | | `--pitch-contours` 内のケースID |
| `--apply-pitch` | `false` | 波形のピッチ再サンプリング（実験的） |
| `--intonation-strength` | `0` | 音源ピッチ安定化と句曲線の強さ（0〜4） |
| `--renderer <id>` | 既定Renderer | Renderer ID（省略時は設定された優先度が最大のもの。未知の明示IDはエラー） |
| `--resampler <id>` | 自動選択 | Classic UTAUで使う`Resamplers/`からの相対ID |
| `--wavtool <id>` | `builtin` | Classic UTAUで使う`Wavtools/`からの相対ID |
| `--resampler-expressions <path>` | | unit単位のresampler設定JSON |
| `--worldline <path>` | 実行ファイルの隣 | OpenUtau worldlineライブラリ |
| `--worldline-bridge <path>` | | `utautts-worldline-bridge` 実行ファイル |
| `--boundary-bridge-ms` | `0` | 位相を揃えた波形接続修復の最大幅（0で無効） |
| `--boundary-bridge-threshold` | `0` | handcrafted join scoreがこの値以下のとき接続修復を適用 |
| `--selection` | `viterbi` | 原音選択方式（`viterbi`、`greedy`、`target-only`） |
| `--alias-policy` | `auto` | 音源適応モード。`auto`はVC/VCV収録比から自動選択、`legacy`はv0.0.9互換、`cvvc-enhanced`はCVVC優先・sequential timing・VC音量35%。詳細指定として`vcv-prefer`、`cvvc-prefer`、`cv-only`も利用可能 |
| `--cvvc-timing` | `legacy` | CVVC遷移の配置方式（`legacy`または`sequential`）。通常は`--alias-policy`でまとめて指定 |
| `--cvvc-transition-gain` | `1` | CVVC遷移ユニットの音量（0〜1） |
| `--cvvc-pre-boundary-fade` | `false` | 後続CVの子音より前でCVVC遷移をフェードアウト |
| `--acoustic-selection` | | 音響特徴による候補選択の診断（`dry-run`または`apply`）。通常利用では空欄 |
| `--join-model <path>` | | 学習済みjoin-costモデルJSON |
| `--join-scale` | `0` | 学習済みlogitスコアの倍率（既定はモデル値または4） |
| `--renderer-dir <dir>` | | Renderer pluginの検索directory（繰り返し指定可） |
| `--model-dir <dir>` | | モデルJSONの検索directory（繰り返し指定可） |
| `--openjtalk-features <path>` | runtime | Open JTalk feature helper（自動検出を上書き） |
| `--openjtalk-dictionary <path>` | runtime | Open JTalk辞書ディレクトリ（自動検出を上書き） |
| `--write-text` | `false` | WAVと同名のTXTを書き出す |
| `--write-lab` | `false` | WAVと同名のHTK形式LABを書き出す |
| `--text-encoding` | `utf-8` | TXTの文字コード（`utf-8`または`shift_jis`） |

## モデルとRendererの指定

`--prosody`と`--renderer`にはファイルパスではなくIDを指定します。Classic UTAUのツールも絶対パスではなく、`Resamplers/`または`Wavtools/`からの相対IDを指定します。一覧は[モデル／Rendererプラグイン](plugins.md)を参照してください。

モデルやRendererは実行ファイルの隣にある`models/`と`renderer/`から自動検出します。別のdirectoryを追加するなら`--model-dir`または`--renderer-dir`で指定します。明示したRenderer定義は同梱定義より優先されます。

`--renderer`を省略した場合はカタログの`default_priority`が最大のRendererを使います。存在しないIDを明示した場合はエラーになります。指定したRendererのassetが不足している場合もエラーになります。

`--resampler-expressions`のJSONは[Classic UTAU互換仕様](plugins.md#classic-utau互換仕様)を参照してください。

`--apply-pitch`と`--intonation-strength`による直接的なピッチ加工は声質と明瞭度を損なう場合があります。

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
