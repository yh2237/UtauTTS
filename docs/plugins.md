# モデル／Rendererプラグイン

Renderer、Classic UTAUツール、抑揚モデルを追加または配布するための仕様を説明します。

モデルとRendererは安定したIDでGUI、CLI、Serverから共通に選びます。実行ファイルの隣にある`models/`と`renderer/`を自動検出し、CLIとServerでは`--model-dir`／`--renderer-dir`で探索先を追加できます。明示した探索先は同梱定義より優先されます。

## Renderer manifest の仕様

標準Rendererも外部Rendererも、`renderer/<id>/renderer.json`で定義します。表示情報、既定値、runtimeのパスはmanifestに記録します。`id`はプロジェクトやAPIに保存する公開IDです。`provider`は実装を選ぶID、`contract`は入力形式の契約を表します。内蔵ProviderはGo側のレジストリで管理し、外部実装は`utautts-provider`プロトコルで接続します。新しい合成エンジンABIはGo側へ実装します。

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

`resources`の`path`はRendererディレクトリ基準です。OSごとに異なる場合は`platform_resources`の`windows-amd64`／`linux-amd64`／`darwin-arm64`などへ同じresource keyを記述できます。`required`と`executable`は宣言情報で、実行時の必須resourceと機能はProviderレジストリとの整合性も検証されます。

`default_priority`が大きいRendererが既定値です。未知のproviderや壊れたmanifestは`problems`へ表示し、その定義だけを無効にします。未知のIDはエラーとして扱います。

配布側が更新・削除を管理する同梱定義には`update_managed: true`を付けます。ユーザーが追加する定義では省略してください。

共有runtimeはパッケージ直下の`runtime/`に置き、manifestからはRendererディレクトリを基準とする相対パスで参照します。OSごとに名前が異なる場合は、`platform_resources`に`windows-amd64`／`darwin-arm64`などを記述します。

対応する内蔵Providerアダプターは`waveform`、`utautts-world-phrase`、`utau-external-resampler`、`diffsinger`です。標準定義の追加やユーザー定義によって、既存アダプターを別の公開IDで選べます。新規エンジンABIの動的ロードには対応していません。

配布プロファイルによって利用できるmanifestとruntimeが異なります。

| 配布物 | 利用できるRenderer |
| --- | --- |
| Windows Full | `utautts-world-phrase`、`waveform`、`classic-utau`、`diffsinger` |
| Windows Japanese | `utautts-world-phrase`、`waveform`、`classic-utau` |
| Linux x64 | `utautts-world-phrase`、`waveform`、`classic-utau` |
| macOS arm64 | `utautts-world-phrase`、`waveform`、`classic-utau` |

WindowsのFullプロファイルだけがDiffSingerのruntimeを含みます。LinuxとmacOSのDiffSinger manifestは対応OS外なのでカタログから除外されます。

Rendererの追加・更新はZIPインストールでは行いません。`renderer/<id>/renderer.json`を探索先へ配置してからGUIを再起動（またはCLI／Serverを再起動）してください。既存IDを明示ディレクトリに置くと同梱定義を上書きできます。

## Renderer設定と機能

manifestの`settings`には、そのRendererが受け付ける設定項目を宣言します。宣言した項目はGUIの設定ウィンドウにRendererごとのタブとして表示され、そのRendererの設定として`config.ini`へ保存されます。CLIとServerからも同じIDで指定できます。

設定は合成リクエストの`renderer_settings`マップとして渡されます。ただし、リクエストのトップレベル項目（`mora_duration_ms`や`intonation_strength`など）と同じIDの設定は、カード単位の値を優先するためこのマップには含まれません。GUIでは宣言した設定が新しいカードの既定値になり、カードごとの設定がそれを上書きします。

```json
{
  "settings": [
    { "id": "mora_duration_ms", "type": "number", "group": "timing", "default": 120, "min": 0, "max": 1000, "label": "モーラ長" },
    { "id": "context_duration", "type": "boolean", "group": "quality", "default": true, "label": "文脈に応じたモーラ長" },
    { "id": "resampler", "type": "enum", "group": "classic", "options_source": "resamplers", "label": "Resampler" }
  ]
}
```

各項目は`id`と`type`（`integer`／`number`／`boolean`／`enum`／`string`）を持ち、`default`、`min`／`max`／`step`、`label`を付けられます。`group`は設定ウィンドウ内の分類です。`enum`では`options`で選択肢を列挙するか、`options_source`に`resamplers`／`wavtools`を指定して対応フォルダの実行ファイルを選ばせます。

`capabilities`にはRendererの機能を宣言します。コアはこの機能フラグを見て、Renderer名ではなく能力に応じて処理を分けます。

| capability | 意味 |
| --- | --- |
| `frame_pitch` | 10 ms単位のフレームピッチ曲線を受け付ける |
| `boundary_bridge` | 境界補修（boundary bridge）に対応する |
| `internal_timing` | 内部でタイミングを調整する（日本語のリズム補正を常時適用する） |
| `speech_prosody_experiment` | 多言語のスピーチ韻律実験（timing/pitch）に対応する |

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

GUIではbackend／providerが`utau-external-resampler`のRendererを選択した場合だけ、ResamplerとWavtoolの欄を表示します。したがって`classic-utau`以外の公開IDでもClassic UTAUを利用できます。外部wavtoolを使わない場合は`builtin`を選びます。配置後は「Classic UTAUを再読み込み」を選びます。

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

複数の実行ファイルを同じ条件で診断する場合は`resampler-compat`を使います。`--mode direct`は13引数の直接呼び出し、`--mode integration`はUtauTTSのPlanと内蔵接続処理まで含む統合検査です。結果は終了状態、WAV形式、長さ、peak、RMSを含むJSONで出力されます。

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
  "license_notice": "licenses/MY-MODEL.txt",
  "provenance": {
    "training_corpus": "Describe the training data",
    "source_notice": "licenses/MY-MODEL-SOURCE.txt"
  },
  "recommended_renderers": ["utautts-world-phrase"],
  "default_priority": 100,
  "version": 8,
  "feature_version": 1,
  "mode": "intonation_frame_tcn_accent_bounded"
}
```

`id`と`display_name`がないJSONはモデルとして扱いません。同じIDや壊れたJSONは診断へ表示します。CLIの`--prosody`にはファイルpathではなくIDを指定します。

既存のJSONモデルを`models/`へ登録する場合は、識別情報とライセンス情報が必須です。`license_notice`と`provenance`には実際の配布条件と出典を記録します。

識別情報のない学習結果には、登録前に次のスクリプトでIDと表示名を付けます。

```powershell
.\tools\install-prosody-model.ps1 `
  -ModelPath .\out\prosody\my-model.json `
  -Id my-model-v1 `
  -DisplayName "My intonation model" `
  -DestinationDirectory .\models
```

## 外部Providerプロトコル v1

manifest v2の`protocol`に`utautts-provider`を指定すると、Rendererの実装を別プロセスとして導入できます。アプリはProviderをshell経由ではなく、`provider_executable`に指定された実行ファイルへ直接起動します。`provider_args`はそのまま引数として渡され、暗黙の`PATH`探索やshell展開は行いません。

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

Providerは起動直後に`hello`を1行返し、protocol version、Provider ID/version、session対応、capability、実装するcontract/versionを宣言します。必須capabilityが不足している場合は起動を拒否します。`unit-renderer` v1の必須job envelopeは`unit_renderer_job_v2`です。session対応Providerは同じプロセスで複数の`render` requestを順番に処理します。通常経路はこのsessionを再利用するため、モデルやruntimeの初期化を合成ごとに繰り返しません。`progress`、`diagnostic`、`result`、`error`、`cancel`、`shutdown`がv1の基本メッセージです。stdoutはNDJSON protocol専用で、診断用の自由なログはstderrへ書きます。

sessionの寿命はProvider processと同じです。Provider processが落ちた場合はpoolから破棄して次回の合成で再起動し、アプリ終了時やvoicebank／model cacheの破棄時には明示的にshutdownします。WORLD bridgeとDiffSinger bridgeもこのsession経路で接続します。

`unit-renderer` v1の`render` requestは、hostが作成したjob directory内の`input_path`と`output_path`を受け取ります。input JSONはjob version 2の`version`、`contract`、`contract_version`、選択済み`plan`、型付き`options`、宣言済み`resources`を持ちます。`provider_payload`は使用せず、Providerは共通フィールドと自分の型付きoptionsを直接消費します。Providerはjob directory内のoutput pathへ16-bit PCM WAVを書き、`result.audio.path`でそのファイルを返します。

同梱WORLD bridgeは共通`unit-renderer` jobの`plan`／`options`／`resources`を受け取ります。WORLD固有の準備済み入力は`options.worldline`に型付きで格納されます。bridgeとhostは同じjob versionのリリースを組み合わせます。

`neural-synthesizer` v1のjobは`version`、`contract`、`contract_version`、共通`score`、provider固有の`options`、宣言済み`resources`を持ちます。`score`は`symbols`、frame duration、F0、MIDI、word grouping、note rest、pitch predictor使用有無を表します。DiffSingerは現段階では`options`に既存のbridge Request形式を入れるadapterですが、モデルpathは`resources`へ分離し、transport上は共通NeuralScore jobとして検証できます。

概念上のjob形状は次の通りです。`options`の内部schemaはproviderごとに異なりますが、`score`と`resources`の位置はcontract v1で共通です。

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

リリースビルドでは`renderer/`と`models/`をGUI版・Server版へコピーします。モデルが一つもない場合はビルドに失敗します。`models/`へ登録するモデルJSONには`license`と`license_notice`が必須です。`license`はモデルの配布条件、`provenance`は学習元と出典を表します。上流データの条件は`license_notice`または`provenance`の通知へ記録します。`license_notice`は配布物のルートからの相対パスで、リポジトリの`licenses/`以下に実在するファイルを指定します。各モデル、Renderer、外部アセットの条件と出典は[ライセンスの適用範囲](../LICENSE-SCOPE.md)、[第三者通知](../THIRD_PARTY_NOTICES.txt)、配布元の文書を参照してください。
