# モデル／Rendererプラグイン

モデルとRendererは安定したIDでGUI、CLI、Serverから共通に選びます。実行ファイルの隣にある`models/`と`renderer/`を自動検出し、CLIとServerでは`--model-dir`／`--renderer-dir`で探索先を追加できます。明示した探索先は同梱定義より優先されます。

## Renderer manifest

標準Rendererも外部Rendererも、`renderer/<id>/renderer.json`で定義します。Go側にRendererの表示情報、既定値、runtime pathをハードコードするレジストリはありません。`id`はプロジェクトやAPIに保存する公開IDです。v1では`backend`、v2では`provider`が実装を選ぶIDで、v2では`contract`が入力契約を表します。providerの内蔵実装はGo側のregistryで管理し、外部実装は`utautts-provider` protocolで接続します。JSONだけで新しい合成エンジンABIを追加する仕組みではありません。

旧形式（v1）の最小manifestは次の形式です。既存のユーザー定義を読むために引き続き対応します。

完全な形式は[renderer.schema.json](renderer.schema.json)を参照してください。実行時は`manifest_version: 1`と、contract／providerを明示する現行の`manifest_version: 2`を読み込めます。新しい定義にはv2を使い、v1は旧ユーザー定義を読むためだけに残します。旧`plugin.json`は起動時には読み込まず、アプリ内アップデーターが旧配置からv1の`renderer.json`へ変換します。過去に存在したpackage v2（`protocol_version`と`platforms`を持つ形式）は現行v2とは別形式として、アップデーターでv1へ変換します。

```json
{
  "manifest_version": 1,
  "kind": "renderer",
  "id": "example.renderer",
  "display_name": "Example Renderer",
  "backend": "waveform",
  "version": "1",
  "capabilities": { "frame_pitch": true }
}
```

新しい定義ではv2を使います。公開IDとprovider、入力contract、provider versionを分離し、runtimeをtyped resourceとして記述します。

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

`resources`の`path`はRendererディレクトリ基準です。OSごとに異なる場合は`platform_resources`の`windows-amd64`／`linux-amd64`へ同じresource keyを記述できます。`required`と`executable`は宣言情報で、実行時の必須resourceとcapabilityはprovider registryとの整合性も検証されます。

`default_priority`が大きいRendererが既定値です。`capabilities`、`acceleration`、`experimental`、v1の`assets`／`platform_assets`、v2の`resources`／`platform_resources`もmanifestに記述します。未知のbackend／providerや壊れたmanifestは`problems`へ表示され、その定義だけが無効になります。未知のIDを別Rendererへ黙って切り替えることはありません。

配布側が更新・削除を管理する同梱定義には`update_managed: true`を付けます。ユーザーが追加する定義では省略してください。

共有runtimeはパッケージ直下の`runtime/`に置き、manifestからはRendererディレクトリ基準の相対pathで参照します。WindowsとLinuxで名前が異なる場合は、v2では`platform_resources`に`windows-amd64`／`linux-amd64`を記述します。次の例はv1のlegacy manifestです。

```json
{
  "assets": {},
  "platforms": ["windows-amd64", "linux-amd64"],
  "platform_assets": {
    "windows-amd64": {
      "world_engine": "../../runtime/utautts-world-engine.dll"
    },
    "linux-amd64": {
      "world_engine": "../../runtime/utautts-world-engine.so"
    }
  }
}
```

対応する内蔵provider adapterは`waveform`、`utautts-world-phrase`、`openutau-worldline-r-faithful`、`utau-external-resampler`、`diffsinger`です。標準定義の追加やユーザー定義によって、既存adapterを別の公開IDで選べます。任意の新規engine ABIを動的ロードする機能はまだありません。

配布プロファイルによって利用できるmanifestとruntimeが異なります。

| 配布物 | 利用できるRenderer |
| --- | --- |
| Windows Full | `utautts-world-phrase`、`openutau-worldline-r-faithful`、`waveform`、`classic-utau`、`diffsinger`（試験実装） |
| Windows Japanese | `utautts-world-phrase`、`waveform`、`classic-utau` |
| Linux x64 | `utautts-world-phrase`、`openutau-worldline-r-faithful`、`waveform`、`classic-utau` |

WindowsのFullプロファイルだけがWORLDLINE-RとDiffSingerのruntimeを含みます。LinuxのDiffSinger manifestは対象プラットフォーム外なのでカタログから除外されます。`utautts-world-phrase-cuda`は評価用の実験定義で、どのリリースZIPにも含めません。

Rendererの追加・更新はZIPインストールでは行いません。`renderer/<id>/renderer.json`を探索先へ配置してからGUIを再起動（またはCLI／Serverを再起動）してください。既存IDを明示ディレクトリに置くと同梱定義を上書きできます。

旧版からアプリ内アップデーターで更新する場合、`plugins/renderers/`にあった外部定義は一度だけ新形式へ変換されます。すでに`renderer/`へ置いたユーザー定義も、新しい配布物に同じIDがなければ引き継がれます。同じIDの標準定義が更新に含まれる場合は、新しい標準定義を優先します。手作業で旧版を上書きする場合、この移行は行われません。

## Classic UTAUツール

resamplerは`Resamplers/`、wavtoolは`Wavtools/`へ配置します。Renderer manifestは不要です。サブディレクトリも探索するため、依存DLLを実行ファイルと同じディレクトリへ置けます。プロジェクトには各ディレクトリからの相対IDを保存します。

```text
Resamplers/
  moresampler.exe
  L2R/
    L2R.exe
    dependency.dll
Wavtools/
  wavtool.exe
```

GUIではbackend／providerが`utau-external-resampler`のRendererを選択した場合だけ、ResamplerとWavtoolの欄を表示します。したがって`classic-utau`以外の公開IDでもClassic UTAUを利用できます。外部wavtoolを使わない場合は`builtin`を選びます。配置後は「Classic UTAUツールを再読み込み」を選びます。

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

複数の実行ファイルを同じ条件で診断する場合は`resampler-compat`を使います。`--mode direct`は13引数の直接呼び出し、`--mode integration`はUtauTTSのPlanと内蔵接続処理まで含む試験です。結果は終了状態、WAV形式、長さ、peak、RMSを含むJSONで出力されます。

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
  "recommended_renderers": ["utautts-world-phrase"],
  "default_priority": 100,
  "version": 8,
  "feature_version": 1,
  "mode": "intonation_frame_tcn_accent_bounded"
}
```

`id`と`display_name`がないJSONはモデルとして扱いません。同じIDや壊れたJSONは診断へ表示します。CLIの`--prosody`にはファイルpathではなくIDを指定します。

GUIで集めた手動調整からモデルを作る方法は[手動調整から抑揚モデルを作る](prosody-model-training.md)にあります。既存のJSONを`models/`へ登録する場合は、必要なidentityを付けてから配置してください。

identityのない学習結果には、登録前に次のscriptでIDと表示名を付けます。

```powershell
.\tools\install-prosody-model.ps1 `
  -ModelPath .\out\prosody\my-model.json `
  -Id my-model-v1 `
  -DisplayName "My intonation model" `
  -DestinationDirectory .\models
```

## 外部Provider protocol v1

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

Providerは起動直後に`hello`を1行返し、protocol version、Provider ID/version、session対応、capability、実装するcontract/versionを宣言します。manifestが要求するcapabilityがhandshakeに含まれない場合は起動を受け付けません。`unit-renderer` v1では共通job envelopeを読む`unit_renderer_job_v2`も必須です。session対応Providerは同じプロセスで複数の`render` requestを順番に処理します。通常経路はこのsessionを再利用するため、モデルやruntimeの初期化を合成ごとに繰り返しません。`progress`、`diagnostic`、`result`、`error`、`cancel`、`shutdown`がv1の基本メッセージです。stdoutはNDJSON protocol専用、診断用の自由なログはstderrへ出してください。

sessionはアプリケーションの合成単位ではなくProvider processの寿命に結び付きます。Provider processが落ちた場合はpoolから破棄して次回の合成で再起動し、アプリ終了時やvoicebank／model cacheの破棄時には明示的にshutdownします。現在legacy one-shot fallbackが残っているのは、古いDiffSinger bridgeを使う環境です。WORLD bridgeはjob v2のsession経路だけを受け付けます。

`unit-renderer` v1の`render` requestはhostが作成したjob directory内の`input_path`と`output_path`を受け取ります。input JSONはjob version 2の`version`、`contract`、`contract_version`、選択済み`plan`、型付き`options`、宣言済み`resources`を持ちます。opaqueな`provider_payload`は存在せず、Providerは共通フィールドと自分の型付きoptionsを直接消費します。Providerはoutput pathへ16-bit PCM WAVを書き、`result.audio.path`でそのファイルを返します。job directory外の出力は受け付けません。

同梱WORLDLINE bridgeは共通`unit-renderer` jobの`plan`／`options`／`resources`だけを受け取ります。WORLD固有の準備済み入力は`options.worldline`に型付きで格納され、旧standalone manifest、`provider_payload`、raw manifest起動へのfallbackはありません。bridgeとhostは同じjob versionのリリースを組み合わせてください。

`neural-synthesizer` v1のjobは`version`、`contract`、`contract_version`、共通`score`、provider固有の`options`、宣言済み`resources`を持ちます。`score`は`symbols`、frame duration、F0、MIDI、word grouping、note rest、pitch predictor使用有無を表します。DiffSingerは現段階では`options`に既存のbridge Request形式を入れる移行adapterですが、モデルpathは`resources`へ分離し、transport上は共通NeuralScore jobとして検証できます。旧bridgeしかない環境では従来の1回起動経路へfallbackします。

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

release buildは`renderer/`と`models/`をGUI・Serverへコピーします。モデルが一つもない場合はビルドに失敗します。各モデル、Renderer、外部assetのライセンスは[ライセンス表示](../THIRD_PARTY_NOTICES.txt)と配布元の文書を確認してください。
