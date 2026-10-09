# モデル／Rendererプラグイン

Renderer、Classic UTAUツール、抑揚モデルを追加または配布するための仕様を説明します。

モデルとRendererは、GUI、CLI、Serverで共通の変わらないIDで選びます。実行ファイルの隣にある`models/`と`renderer/`を自動検出し、CLIとServerでは`--model-dir`／`--renderer-dir`で探索先を追加できます。明示した探索先は同梱定義より優先されます。

## Renderer manifest の仕様

標準Rendererも外部Rendererも、`renderer/<id>/renderer.json`で定義します。表示情報、既定値、runtimeのパスはmanifestに記録します。`id`はプロジェクトやAPIに保存する公開IDです。`provider`は実装を選ぶID、`contract`は受け取る入力の形式を表します。内蔵ProviderはGo側のレジストリで管理し、外部実装は`utautts-provider`プロトコルで接続します。新しい合成エンジンABIはGo側へ実装します。

完全な形式は[renderer.schema.json](renderer.schema.json)を参照してください。実行時に読み込むのは`manifest_version: 2`だけです。公開ID、Provider、入力contract、Provider versionを分離し、runtimeを種別付きresourceとして記述します。

```json
{
  "manifest_version": 2,
  "kind": "synthesis-engine",
  "id": "example.world",
  "display_name": "Example WORLD",
  "contract": "unit-renderer",
  "provider": "utautts-world-phrase",
  "provider_version": "1",
  "resources": {
    "world_engine": { "path": "../../runtime/engine.dll", "required": true }
  }
}
```

`resources`の`path`はRendererディレクトリを基準にした相対パスです。共有のruntimeはパッケージ直下の`runtime/`に置き、manifestから相対パスで参照します。OSごとにファイルが異なる場合は、`platform_resources`の`windows-amd64`／`linux-amd64`／`darwin-arm64`などへ同じresource keyで記述します。`required`と`executable`は宣言で、実行時に必要なresourceと機能はProviderレジストリの定義とも照らし合わせて検証します。

`default_priority`が最も大きいRendererが既定になります。未知のproviderや読み込めないmanifestは`problems`へ表示し、その定義だけを無効にします。存在しないRenderer IDを指定した場合はエラーになります。

配布側が更新・削除を管理する同梱定義には`update_managed: true`を付けます。ユーザーが追加する定義では省略してください。

同梱RendererのProviderは`utautts-world-phrase`、`utau-external-resampler`、`diffsinger`です。定義を追加すると、既存のProviderを別の公開IDで選べます。新しいエンジンのABIを実行時に読み込むことには対応していません。

配布プロファイルによって利用できるmanifestとruntimeが異なります。

| 配布物 | 利用できるRenderer |
| --- | --- |
| Windows Full | `utautts-world-phrase`、`classic-utau`、`diffsinger` |
| Windows Japanese | `utautts-world-phrase`、`classic-utau` |
| Linux x64 | `utautts-world-phrase`、`classic-utau` |
| macOS arm64 | `utautts-world-phrase`、`classic-utau` |

DiffSingerのruntimeを含むのはWindowsのFullプロファイルだけです。LinuxとmacOSでは、DiffSingerのmanifestは対応OS外としてカタログから除きます。

Rendererの追加・更新はZIPインストールでは行いません。`renderer/<id>/renderer.json`を探索先へ配置してからGUIを再起動（またはCLI／Serverを再起動）してください。既存IDを明示ディレクトリに置くと同梱定義を上書きできます。読み込んだRendererと読み込めなかった定義の理由は、GUIの「ファイル」→「音源とプラグイン」→「Rendererプラグイン」で確認できます。

## Renderer設定と機能

UtauTTSが解釈する設定（モーラ長、抑揚の強さ、文末の音調、WORLDの時間伸縮、Classicのresamplerなど）は、既定値・範囲・表示と対象のproviderを、Go側の設定表（`internal/settings`）の1か所で定義します。カタログは、設定表のうちそのRendererのproviderに適用される項目をmanifestの設定へ加えます。そのため同梱Rendererのmanifestは`settings`を持ちません。manifestの`settings`には、設定表にない外部provider固有の項目だけを宣言します。設定表と同じIDを宣言すると、そのRendererでは既定値や範囲を上書きできます。

加えた項目はGUIの設定ウィンドウにRendererごとのタブとして表示され、そのRendererの設定として`config.ini`へ保存されます。CLIとServerからも同じIDで指定できます。

設定は合成リクエストの`renderer_settings`マップとして渡されます。ただし、リクエストのトップレベル項目（`mora_duration_ms`や`intonation_strength`など）と同じIDの設定は、カード単位の値を優先するためこのマップには含まれません。GUIでは宣言した設定が新しいカードの既定値になり、カードごとの設定がそれを上書きします。

```json
{
  "settings": [
    { "id": "mora_duration_ms", "type": "number", "group": "timing", "default": 120, "min": 0, "max": 1000, "label": "モーラ長" },
    { "id": "boundary_tone", "type": "boolean", "group": "correction", "default": true, "label": "文末の音調" },
    { "id": "resampler", "type": "enum", "group": "classic", "options_source": "resamplers", "label": "Resampler" }
  ]
}
```

各項目は`id`と`type`（`integer`／`number`／`boolean`／`enum`／`string`）を持ち、`default`、`min`／`max`／`step`、`label`を付けられます。`group`は設定ウィンドウ内の分類です。`enum`では`options`で選択肢を列挙するか、`options_source`に`resamplers`／`wavtools`を指定して対応フォルダの実行ファイルを選ばせます。

`capabilities`にはRendererの機能を宣言します。本体はRenderer名ではなく、このフラグを見て処理を分けます。

| capability | 意味 |
| --- | --- |
| `frame_pitch` | 10 ms単位のフレームピッチ曲線を受け付ける |
| `boundary_bridge` | 境界補修（boundary bridge）に対応する |
| `internal_timing` | 内部でタイミングを調整する（日本語のリズム補正を常時適用する） |

## Classic UTAUツール

resamplerは`Resamplers/`、wavtoolは`Wavtools/`へ配置します。Classic UTAUツールにmanifestはありません。サブディレクトリも探索するため、依存DLLを実行ファイルと同じディレクトリへ置けます。プロジェクトには各ディレクトリからの相対IDを保存します。

```text
Resamplers/
  moresampler.exe
  L2R/
    L2R.exe
    dependency.dll
Wavtools/
  wavtool.exe
```

GUIでは、providerが`utau-external-resampler`のRendererを選んだときにResamplerとWavtoolの欄を表示します。そのため`classic-utau`以外の公開IDでも、Classic UTAUの方式を使えます。外部wavtoolを使わない場合は`builtin`を選びます。配置後は「ファイル」→「音源とプラグイン」→「Classic UTAUを再読み込み」を選びます。

UTAU互換のresampler呼び出しは、入力WAV、出力WAV、音高、velocity、flags、offset、必要長、consonant、cutoff、volume、modulation、tempo、12bit Base64ピッチ列の13引数です。ノート単位の設定はAPIの`resampler_expressions`またはCLIの`--resampler-expressions`で指定できます。

### Classic UTAU互換仕様

外部wavtoolを選んだ場合はOpenUtau Classic Rendererと同じ位置引数で接続し、`builtin`ではUtauTTS内蔵の5点包絡線処理を使います。外部実行ファイルの利用条件は配布元の規約に従ってください。

resamplerの既定値はvelocity 100、空のflags、modulation 0、tempo 120です。`offset`、必要長、consonant、cutoff、volumeを含めた13引数をOpenUTAU互換の順序で渡します。

ノート単位の設定例です。`position`は読みのモーラ位置で、CVVC transitionにも親モーラの設定が引き継がれます。省略した値はRenderer全体の設定を使います。

```json
[
  { "position": 0, "velocity": 86, "volume": 90, "flags": "g-3", "modulation": 4, "tempo": 150 },
  { "position": 2, "flags": "Mt10" }
]
```

複数の実行ファイルを同じ条件で確かめる場合は`resampler-compat`を使います。`--mode direct`は13引数で直接呼び出し、`--mode integration`はUtauTTSのPlanと内蔵の接続処理まで含めて検査します。結果は終了状態、WAV形式、長さ、peak、RMSを含むJSONで出力されます。

```powershell
go run ./cmd/tools/resampler-compat `
  --mode integration `
  --wavtool path/to/wavtool.exe `
  --input sample.wav `
  --out-dir out/resampler-compat `
  path/to/resampler.exe
```

## 抑揚モデル

モデルJSON自身がmanifestを兼ねます。

```json
{
  "id": "my-model-v1",
  "display_name": "My intonation model",
  "license": "MIT License",
  "license_notices": ["licenses/MY-CORPUS-NOTICE.txt"],
  "provenance": {
    "training_corpus": "Describe the training data"
  },
  "recommended_renderers": ["utautts-world-phrase"],
  "default_priority": 100,
  "version": 8,
  "feature_version": 1,
  "mode": "intonation_frame_tcn_accent_bounded"
}
```

`id`と`display_name`がないJSONはモデルとして扱いません。IDの重複や読み込めないJSONは診断へ表示します。CLIの`--prosody`にはファイルのパスではなくIDを指定します。

既存のJSONモデルを`models/`へ登録する場合は、識別情報とライセンス情報が必須です。`license_notices`（使用した各データの通知の配列）と`provenance`には実際の配布条件と出典を記録します。

識別情報のない学習結果には、登録前に次のスクリプトでIDと表示名を付けます。

```powershell
.\tools\install-prosody-model.ps1 `
  -ModelPath .\out\prosody\my-model.json `
  -Id my-model-v1 `
  -DisplayName "My intonation model" `
  -DestinationDirectory .\models
```

## 外部Providerプロトコル v1

manifest v2の`protocol`に`utautts-provider`を指定すると、Rendererの実装を別プロセスとして追加できます。アプリはProviderをシェル経由ではなく、`provider_executable`に指定した実行ファイルとして直接起動します。`provider_args`はそのまま引数として渡し、`PATH`の探索やシェルの展開は行いません。

```json
{
  "manifest_version": 2,
  "kind": "synthesis-engine",
  "id": "example.external-world",
  "display_name": "Example external WORLD",
  "contract": "unit-renderer",
  "contract_version": 1,
  "provider": "example.world",
  "provider_version": "1",
  "protocol": "utautts-provider",
  "protocol_version": 1,
  "provider_args": ["--serve"],
  "resources": {
    "provider_executable": {
      "path": "bin/example-provider.exe",
      "required": true,
      "executable": true
    }
  }
}
```

Providerは起動直後に`hello`を1行返し、プロトコルのバージョン、ProviderのIDとバージョン、セッションに対応するか、capability、実装するcontractとバージョンを宣言します。必須のcapabilityが足りない場合、アプリはセッションの開始をエラーにします。`unit-renderer` v1で必須のjobの形式は`unit_renderer_job_v2`です。

セッションに対応したProviderは、同じプロセスで複数の`render`要求を順番に処理します。通常はこのセッションを使い回すため、モデルやruntimeの初期化を合成ごとに繰り返しません。v1の基本メッセージは`progress`、`diagnostic`、`result`、`error`、`cancel`、`shutdown`です。stdoutはNDJSONのプロトコル専用で、診断用の自由なログはstderrへ書きます。

セッションの寿命はProviderのプロセスと同じです。プロセスが終了した場合は保持しているセッションから外し、次の合成で起動し直します。アプリの終了時や、音源・モデルのキャッシュを破棄するときは明示的に`shutdown`を送ります。WORLD bridgeとDiffSinger bridgeもこのセッションで接続します。

`unit-renderer` v1の`render`要求は、アプリが作ったjobディレクトリ内の`input_path`と`output_path`を受け取ります。入力のJSON（job version 2）は`version`、`contract`、`contract_version`、選択済みの`plan`、型付きの`options`、宣言済みの`resources`を持ちます。`provider_payload`は使わず、Providerは共通のフィールドと自分用の型付き`options`を直接読みます。Providerはjobディレクトリ内の出力パスへ16-bit PCMのWAVを書き、`result.audio.path`でそのファイルを返します。

同梱のWORLD bridgeは、共通の`unit-renderer` jobの`plan`／`options`／`resources`を受け取ります。WORLD固有の準備済みの入力は、型付きで`options.worldline`に入ります。bridgeとアプリは、同じjob versionのリリースどうしで組み合わせてください。

`neural-synthesizer` v1のjobは、`version`、`contract`、`contract_version`、共通の`score`、provider固有の`options`、宣言済みの`resources`を持ちます。`score`は音素記号、フレーム単位の長さ、F0、MIDI、単語のまとまり、休符、ピッチ予測器を使うかどうかを表します。DiffSingerは現時点では`options`に既存のbridgeの要求形式を入れていますが、モデルのパスは`resources`へ分けてあり、受け渡しの上では共通のNeuralScore jobとして検証できます。

jobの形は概念上次のとおりです。`options`の中身はproviderごとに異なりますが、`score`と`resources`の位置はcontract v1で共通です。

```json
{
  "version": 1,
  "contract": "neural-synthesizer",
  "contract_version": 1,
  "score": {
    "symbols": ["SP", "a"],
    "durations": [2, 8],
    "f0": [220, 220, 220, 220],
    "midi": 60,
    "word_div": [2],
    "word_dur": [10],
    "note_rest": [false],
    "use_pitch_predictor": true
  },
  "options": {},
  "resources": { "acoustic_model": "..." }
}
```

## 配布物

リリースビルドでは`renderer/`と`models/`をGUI版・Server版へコピーします。モデルが一つもない場合はビルドに失敗します。`models/`へ登録するモデルJSONには`license`と`license_notices`が必須です。`license`はモデルの配布条件、`provenance`は学習元と出典を表します。`license_notices`は使用した各データ（コーパスなど）の通知を列挙します。各通知は配布物のルートからの相対パスで、リポジトリの`licenses/`以下に実在するファイルを指定します。各モデル、Renderer、外部アセットの条件と出典は[ライセンスの適用範囲](../LICENSE-SCOPE.md)、[第三者通知](../THIRD_PARTY_NOTICES.txt)、配布元の文書を参照してください。
