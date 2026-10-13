# コマンドライン（CLI）

`utautts-cli`で音源・モデル・Rendererを指定し、WAVを生成します。

GUI配布物ではWindowsの`tools/utautts-cli.exe`、Linux／macOSの`tools/utautts-cli`にあります。Server専用ZIPにはCLIを含みません。開発時は`go run ./cmd/utautts-cli`でも実行できます。

## 必須の引数

合成時は次の引数を指定します。一覧取得・環境診断・バージョン表示には不要です。

- `--voicebank <dir>`: UTAUボイスバンクのディレクトリ
- `--out <path>`: 出力WAVのパス。`-`で標準出力へ出す
- `--text <文>`または`--reading <読み>`: 合成する文章または読み。UTF-8ファイルや標準入力からも読み込める

`--text`は選択した言語と発音形式で読みを生成します。`--reading`はかな、ARPAbet、Pinyinなどの読みを直接指定します。`--kana`は互換用の別名です。日本語以外を使う場合は`--language`も指定してください。

## 基本例

既定の`utautts-world-phrase` Rendererを使う最小例です。

```powershell
.\UtauTTS\tools\utautts-cli.exe `
  --voicebank ".\UtauTTS\voice\足立レイver3.5.0" `
  --text "あらゆる現実をすべて自分のほうへねじ曲げたのだ。" `
  --renderer utautts-world-phrase `
  --out ".\out.wav"
```

抑揚モデルを使う場合は`--prosody`にモデルIDを指定します。省略するとモデルを使いません。

```powershell
.\UtauTTS\tools\utautts-cli.exe `
  --voicebank ".\UtauTTS\voice\足立レイver3.5.0" `
  --text "あらゆる現実をすべて自分のほうへねじ曲げたのだ。" `
  --prosody frame-intonation-tcn-v12 `
  --out ".\out.wav"
```

`--plan-out`を指定すると原音の配置とタイミングをJSONへ保存できます。

英語では`--language en --prosody frame-intonation-tcn-en-v1`、中国語では`--language zh --prosody tone-intonation-zh-v1`を指定します。各言語のモデルは`utautts-world-phrase`で抑揚に反映されます。

GUIと同じユーザー辞書を使うには、次のJSONを`--dictionary dictionary.json`で渡します。

```json
[
  {"surface": "v8", "reading": "ぶいはち"},
  {"surface": "UtauTTS", "reading": "うたうてぃーてぃーえす"}
]
```

`--write-text`または`--write-lab`を指定すると、WAVと同じ場所へ同名の`.txt`または`.lab`を保存します。

## ファイル・標準入力から読む

`--text-file <path>`で文章、`--reading-file <path>`で読みをUTF-8ファイルから取り込みます。パスに`-`を指定すると標準入力をEOFまで読みます。

```sh
utautts-cli --voicebank "voice/足立レイver3.5.0" --text-file message.txt --out speech.wav

printf '%s\n' 'こんにちは。' | utautts-cli --voicebank "voice/足立レイver3.5.0" --text-file - --out speech.wav
```

先頭のUTF-8 BOMと末尾の改行は除き、途中の改行は保持します。ファイル・標準入力の上限はそれぞれ1 MiBです。空白だけの入力やUTF-8以外の入力はエラーになります。

`--text-file`と`--text`、`--reading-file`と`--reading`／`--kana`は併用できません。文章と読みを両方渡す場合は従来どおり読みを優先します。標準入力を文章と読みの両方へ割り当てることはできません。

## JSON要求から合成する

`--request <path>`でUTF-8のJSON要求を読み込みます。`--request -`は標準入力を使います。入力は1個のJSONオブジェクトで、BOM・入力サイズの扱いは文章ファイルと同じです。

```json
{
  "text": "こんにちは。",
  "renderer": "utautts-world-phrase",
  "model_id": "frame-intonation-tcn-v12",
  "mora_duration_ms": 120,
  "apply_pitch": true,
  "renderer_settings": {"timing_warp": true}
}
```

```sh
utautts-cli --voicebank "voice/足立レイver3.5.0" --request request.json --out speech.wav --json
```

項目・既定値・優先順は[HTTP APIの合成要求](server.md#リクエスト項目)と共通です。通常の設定は直下、方式固有の設定は`renderer_settings`へ指定します。`kana`などの互換入力や、モーラ長・手動ピッチにも対応します。音源は`voicebank_id`ではなく`--voicebank`へパスを指定してください。

JSON要求と発話フラグ（`--text`・`--reading`・`--renderer`・`--prosody`・`--mora-ms`など）は併用できません。配置の指定、`--out`・`--plan-out`・`--ustx-out`、付随ファイルの指定、`--json`は併用できます。評価用の特徴量や固定曲線など、JSON形式にないCLI専用入力は従来のフラグで使います。

未知の要求項目や複数オブジェクトは入力エラーです。`renderer_settings`内の未知IDはRendererへ渡し、型不一致などは既存のHTTP入力と同じ診断規則で扱います。休止のゼロは既定値扱い、リリースの明示ゼロには`release_set: true`を使う互換規則も共通です。

## 音源・モデル・Rendererの一覧

`--list`は合成せずに一覧を返します。`--out`や発話の指定は不要で、併用すると入力エラーになります。

```sh
utautts-cli --list voicebanks --voicebank voice
utautts-cli --list models --json
utautts-cli --list renderers --json
utautts-cli --list resamplers
utautts-cli --list wavtools
```

音源の一覧では`--voicebank`を探索元として使い、単一音源のディレクトリも指定できます。省略時は既存の`voice/`探索規則を使います。モデル・Rendererの追加探索先は`--model-dir`・`--renderer-dir`、WORLD bridgeの配置は`--worldline-bridge`で指定できます。

`--json`で`ok: true`と`catalog`を返します。`catalog.kind`は一覧の種類、`items`は要素、`problems`は読込警告です。該当項目がなくても空配列で成功します。

| 項目 | 内容 |
| --- | --- |
| `id`・`name` | IDと表示名 |
| `path` | 音源・Rendererのディレクトリ、モデル・Classicツールのファイルパス。該当する場合だけ含む |
| `language`・`voicebank_kind` | モデルの言語、音源の種類。該当する場合だけ含む |
| `provider`・`available`・`problems` | RendererのProviderと資源の利用可否、不足等の説明 |
| `catalog.default_id` | 既定のRendererまたはClassicツール。モデルは既定で使わないため`none` |

一覧取得は利用できないRendererがあっても成功するため、各要素の`available`を確認してください。これはファイル配置の検査結果で、helper・Providerの起動や読み解析・合成の動作は保証しません。

## 環境診断

`--doctor`は合成せず、選択した資源の配置を検査します。`--out`や発話入力とは併用できません。

```sh
utautts-cli --doctor --voicebank "voice/足立レイver3.5.0" --prosody frame-intonation-tcn-v12
utautts-cli --doctor --renderer utautts-world-phrase --json
```

検査対象は選択Rendererの必要資源、Classicツール、モデルID、明示した音源、Open JTalkのhelperファイルと辞書ディレクトリです。探索先や資源パスは合成時と同じフラグで指定できます。カタログの読込警告も返します。

各検査の`status`は`ok`・`warning`・`error`です。未指定の音源と、自動探索で見つからないOpen JTalk資源は警告にします。選択Renderer・モデルや明示した資源に不足があれば終了コード1、警告だけなら0を返します。

`--json`では`doctor.version`・`os`・`arch`と`checks`を返します。各検査は`id`・`status`・`message`、該当する場合は`path`を持ちます。検査エラー時も`doctor`を残し、`ok: false`と`error`を返します。Runtimeの初期化に失敗した場合は検査結果を返せません。

これは静的な配置検査です。音源の波形、辞書の内容、helperの実行権限・起動、Providerの通信、合成結果は検証しません。`ok: true`は全機能の動作保証ではありません。

## 合成のタイムアウト

`--timeout 30s`で合成処理の制限時間を指定します。`500ms`・`2m`などの時間表記を使い、既定の`0`は無制限です。負の値は入力エラーです。JSON要求とも併用できますが、一覧取得・環境診断とは併用できません。

制限時間は入力読込とRuntime初期化後から数えます。合成後・USTX用の追加予測後・完了報告前にも期限を確認し、期限切れは終了コード1、JSONでは`deadline_exceeded`を返します。

stdin待ち、Runtimeの初期化・終了、同期ファイルI/O、stdout書込は強制終了しません。期限切れ後も処理の終了を待つため、指定時間を超える場合があります。保存済みファイルは残ります。プロセス全体の制限時間は呼出し側で管理してください。

## オプション


| オプション | 既定値 | 説明 |
|---|---|---|
| `--version` | | アプリケーションのバージョンを表示して終了 |
| `--json` | `false` | 結果・診断・エラーをJSONで標準出力へ出す。`--list`・`--doctor`・`--version`にも対応 |
| `--list <kind>` | | `voicebanks`・`models`・`renderers`・`resamplers`・`wavtools`の一覧。`--json`対応 |
| `--doctor` | `false` | 選択した資源の配置を静的に検査。`--json`対応 |
| `--timeout <duration>` | `0` | 入力読込・初期化後の合成処理の制限時間。例: `30s`。`0`で無制限 |
| `--voicebank <dir>` | | ボイスバンクのディレクトリ（必須） |
| `--text <文>` | | 合成する文章 |
| `--text-file <path>` | | UTF-8の文章ファイル。`-`で標準入力 |
| `--reading <読み>` | | かな、ARPAbet、またはPinyinを直接指定 |
| `--reading-file <path>` | | UTF-8の読みファイル。`-`で標準入力 |
| `--request <path>` | | HTTPと共通形式のJSON要求。`-`で標準入力。発話フラグとは併用不可 |
| `--kana <読み>` | | `--reading`と同じ入力を受け付ける互換用の別名 |
| `--language <id>` | `ja` | 言語。`ja`、`en`、`zh` |
| `--phonemizer <id>` | 言語から自動選択 | phonemizer。`ja-kana`、`en-arpasing`、`en-delta`、`en-vccv`、`en-cv`、`zh-cvvc` |
| `--tone` | `C4` | `prefix.map` 使用時に使う音階 |
| `--color <name>` | | `character.yaml`で定義された音源タイプ／サブバンク |
| `--out <path>` | | 出力WAVのパス（必須）。`-`で標準出力 |
| `--plan-out <path>` | | 合成計画JSONを保存するパス |
| `--ustx-out <path>` | | 合成パラメータをOpenUtauのUSTXプロジェクトへ保存するパス |
| `--dictionary <path>` | | 表記と読みを定義したユーザー辞書JSON |
| `--mora-ms` | `120` | 基本モーラ長（ms） |
| `--pause-ms` | `180` | 句読点の休止長（ms） |
| `--mora-durations <path>` | | モーラごとの長さを配列または`mora_durations_ms`で持つJSON |
| `--leading-preutterance-ms` | `0` | 文頭に確保する先行発声（ms）。0では`oto.ini`から自動決定 |
| `--release-ms` | `20` | ユニット末尾のリリース包絡線（ms） |
| `--prosody <id>` | | 抑揚モデルのplugin ID |
| `--prosody-pitch-only` | `false` | 学習ピッチのみ適用し、モーラ長・音量は固定値を使う |
| `--manual-pitch <path>` | | [手動ピッチ編集JSON](server.md#手動ピッチ編集json) |
| `--join-model <path>` | | 学習済みの接続モデルJSON（[モデルの学習](model-training.md#接続モデル)） |
| `--prosody-features <path>` | | ケース別のモーラ単位アクセント特徴JSON |
| `--prosody-feature-case <id>` | | `--prosody-features` 内のケースID |
| `--pitch-contours <path>` | | ケース別ピッチ係数JSON（計画へ記録。波形処理には `--apply-pitch` が必要） |
| `--pitch-case <id>` | | `--pitch-contours` 内のケースID |
| `--apply-pitch` | `true` | Rendererのピッチ処理を有効にする |
| `--intonation-strength` | `4` | 自動抑揚の強さ（0〜8）。v12では曲線に「強さ/4」を掛ける。詳細は[抑揚の強さ](technical-design.md#抑揚の強さ) |
| `--context-duration` | `false` | 日本語のモーラ長を文脈（助詞・句末など）に応じて変える |
| `--context-duration-strength` | `1` | 文脈連動の強度（0〜2）。0は既定1.0として扱う |
| `--boundary-tone` | `true` | 日本語の文末の音調（疑問文の上昇など）を付ける |
| `--boundary-tone-strength` | `1` | 境界音調の強度（0〜2）。0は既定1.0として扱う |
| `--stretch-adapt` | `true` | 原音より大きく伸ばす日本語の長いモーラで、母音側の伸びを音源の実測に合わせて抑える |
| `--stretch-adapt-strength` | `1` | 伸縮補正の強度（0〜2）。0は既定1.0として扱う |
| `--pause-context` | `true` | 句読点の種類と発話末で休止長を変える |
| `--pause-context-strength` | `1` | 休止長補正の強度（0〜2）。0は既定1.0として扱う |
| `--english-weak-form` | `true` | 英語の機能語を句中では弱形で読む。明示の読みと辞書を優先する |
| `--microprosody` | `true` | 日本語のWORLD合成で、子音の前後の微細な音高変化を補う（母音開始の−20〜+40 ms） |
| `--timing-warp` | `true` | 日本語・英語・中国語のWORLD合成で、学習した時間配分に合わせてモーラ内を伸縮する。包絡・開始時刻・長さ・音高は保ち、句末モーラは伸縮しない |
| `--renderer <id>` | 既定Renderer | Renderer ID（省略時は設定された優先度が最大のもの。未知の明示IDはエラー） |
| `--resampler <id>` | 自動選択 | Classic UTAUで使う`Resamplers/`からの相対ID |
| `--wavtool <id>` | `builtin` | Classic UTAUで使う`Wavtools/`からの相対ID |
| `--resampler-expressions <path>` | | unit単位のresampler設定JSON |
| `--worldline-bridge <path>` | | `utautts-worldline-bridge` 実行ファイル |
| `--alias-policy` | `auto` | 音源適応モード。`auto`はVC/VCV収録比から自動選択、`cvvc-enhanced`はCVVC優先・sequential timing・VC音量35%（英語では55%）。詳細指定として`vcv-prefer`、`cvvc-prefer`、`cv-only`も利用可能 |
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

モデルやRendererの追加先は`models/`と`renderer/`です。別の探索先は`--model-dir`／`--renderer-dir`で指定し、明示したRenderer定義を同梱定義より優先します。

`--renderer`を省略した場合はカタログの`default_priority`が最大のRendererを使います。存在しないIDを明示した場合はエラーになります。指定したRendererの必要なファイルが不足している場合もエラーになります。

`--resampler-expressions`のJSONは[Classic UTAU互換仕様](plugins.md#classic-utau互換仕様)を参照してください。

## 出力

成功するとWAVを書き出して次の行を表示します。`--write-text`と`--write-lab`の内容はGUIから保存した場合と同じです。

```
wrote out.wav (4.81s, 44100 Hz, 34 units)
```

`--out -`はWAVだけを標準出力へ出し、完了表示を標準エラーへ出します。合成完了後にWAV全体を出力する方式で、合成途中のストリーミングではありません。

```sh
utautts-cli --voicebank "voice/足立レイver3.5.0" --text-file message.txt --out - > speech.wav
```

WAVを標準出力へ出す場合、`--write-text`と`--write-lab`は使えません。`--plan-out`と`--ustx-out`へのファイル保存は併用できます。

Windows PowerShell 5.1など、外部コマンドの出力を文字列へ変換する環境ではWAVのリダイレクトを避けてください。`--out speech.wav`で直接保存するか、バイナリ出力を保つPowerShell 7.4以降などを使ってください。

合成・入出力・必須引数のエラーは終了コード1、未知のフラグなどフラグ解析のエラーは2を返します。原音の選択と配置は`--plan-out`で確認できます。

## JSON結果とエラー

`--json`を指定すると、通常の完了表示に代えて1個のJSONオブジェクトを標準出力へ出します。WAVは`--out`のファイルへ保存し、PCMサンプルはJSONには含めません。`--out -`とは併用できません。

```sh
utautts-cli --json --voicebank "voice/足立レイver3.5.0" --text-file message.txt --out speech.wav
utautts-cli --json --version
```

成功時は`ok: true`と`result`を返します。

| `result`の項目 | 内容 |
| --- | --- |
| `path` | 指定したWAV保存先 |
| `reading`・`renderer_id` | 合成に使った読みとRenderer ID |
| `duration_ms`・`sample_rate`・`channels` | 音声の長さ（ms）、サンプルレート（Hz）、チャンネル数 |
| `unit_count` | 合成計画のユニット数 |
| `phoneme_labels` | LABと同じ文字列。各行は開始・終了時刻（100 ns単位）と音素。計画由来で、波形の強制アライメントではない |
| `diagnostics` | 診断の配列。`message`と、指定されていれば`severity`・`code`を含む。診断なしは空配列 |

音の不足などの警告があっても合成が完了すれば`ok: true`です。`diagnostics`も確認してください。`--version --json`では`result`に代えて`version`内に`name`と`version`を返します。

失敗時は`ok: false`と`error.code`・`error.message`を返します。

```json
{"ok":false,"error":{"code":"invalid_arguments","message":"--voicebank and --out are required"}}
```

| `error.code` | 分類 |
| --- | --- |
| `invalid_arguments` | 必須引数、併用できない指定、設定の解決に関するエラー |
| `input_error` | 入力ファイル・標準入力・辞書・編集入力の読込エラー |
| `unavailable` | 音源・Renderer・モデルなどを利用できないと判定したエラー |
| `initialization_error` | Runtimeの初期化エラー |
| `synthesis_error` | 読み解析・合成処理のエラー |
| `output_error` | WAV・付随ファイル・結果JSONの出力エラー |
| `shutdown_error` | Runtimeの終了エラー |
| `canceled`・`deadline_exceeded` | 合成要求のキャンセル・タイムアウト |

フラグ解析が成功してからJSON出力を有効にします。未知のフラグ、不正なフラグ値、`--help`ではJSONを出さず、標準エラーへ説明を出します。stdoutへの書込自体が失敗した場合もJSONを返せないことがあります。

JSONの成否はRuntimeの終了後に確定し、失敗しても保存済みファイルは残ります。終了コードはフラグ解析エラーで2、それ以外で1です。分類は`error.code`で判定します。

## 手動ピッチとモーラ長

GUIで編集した抑揚は、[手動ピッチ編集JSON](server.md#手動ピッチ編集json)を`--manual-pitch`へ渡すと再現できます。手動ピッチを波形へ反映するには`--apply-pitch`（既定で有効）が必要です。

```powershell
.\UtauTTS\tools\utautts-cli.exe `
  --voicebank ".\UtauTTS\voice\足立レイver3.5.0" `
  --text "こんにちは" `
  --prosody frame-intonation-tcn-v12 `
  --manual-pitch ".\manual-pitch.json" `
  --out ".\manual-pitch.wav"
```

モーラごとの長さは、読みの順に並べた配列、または`{"mora_durations_ms": [...]}`形式のJSONを`--mora-durations`へ指定します。0の位置は基本長を使います。

## USTXへの一括変換

保存済みの`.utautts`プロジェクトは、配布物の`tools/utautts-ustx.exe`または`tools/utautts-ustx`でOpenUtauのUSTXへ変換できます。開発時は`go run ./cmd/tools/utautts-ustx`でも実行できます。

```console
utautts-ustx project.utautts project.ustx
```

出力パスを省略すると、元のプロジェクトと同じ場所へ`<名前>.ustx`として保存します。GUIの「USTXとして書き出す」では、解析前のカードも書き出すときに解析します。

保存済みの自動フレームピッチ曲線を優先し、手動のフレーム補正またはモーラ単位の補正を加えて出力します。GUIでは保存曲線がない発話だけ再予測を試みます。OpenUtau側の音素化・原音選択・Rendererが異なるため、合成音声の完全一致は保証しません。
