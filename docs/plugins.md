# モデル／Rendererプラグイン

モデルとRendererは安定したIDでGUI、CLI、Serverから共通に選びます。実行ファイルの隣にある`models/`と`renderer/`を自動検出し、CLIとServerでは`--model-dir`／`--renderer-dir`で探索先を追加できます。明示した探索先は同梱定義より優先されます。

## Renderer manifest

標準Rendererも外部Rendererも、`renderer/<id>/renderer.json`で定義します。Go側にRendererの表示情報、既定値、runtime pathを登録するレジストリはありません。`id`はプロジェクトやAPIに保存する公開ID、`backend`はUtauTTSにコンパイル済みの実装を選ぶIDです。JSONだけで新しい合成エンジンABIを追加する仕組みではありません。

最小のmanifestは次の形式です。

完全な形式は[renderer.schema.json](renderer.schema.json)を参照してください。読み込めるのは`renderer.json`の`manifest_version: 1`だけです。旧`plugin.json`とmanifest v2は読み込みません。

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

`default_priority`が大きいRendererが既定値です。`capabilities`、`acceleration`、`experimental`、`assets`、`platform_assets`もmanifestに記述します。未知のbackendや壊れたmanifestは`problems`へ表示され、その定義だけが無効になります。未知のIDを別Rendererへ黙って切り替えることはありません。

配布側が更新・削除を管理する同梱定義には`update_managed: true`を付けます。ユーザーが追加する定義では省略してください。

共有runtimeはパッケージ直下の`runtime/`に置き、manifestからはRendererディレクトリ基準の相対pathで参照します。WindowsとLinuxで名前が異なる場合は`platform_assets`に`windows-amd64`／`linux-amd64`を記述します。

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

対応するbackend adapterは`waveform`、`utautts-world-phrase`、`openutau-worldline-r-faithful`、`utau-external-resampler`、`diffsinger`です。標準定義の追加やユーザー定義によって、既存adapterを別IDで選べます。任意の新規engine ABIを動的ロードする機能はまだありません。

配布物には次のmanifestが含まれます。

- `utautts-world-phrase`: WORLD phrase renderer
- `openutau-worldline-r-faithful`: OpenUTAU WORLDLINE-R faithful renderer
- `waveform`: Go内で原音を伸縮・接続するrenderer
- `classic-utau`: 配置したresamplerとwavtoolを使うrenderer

CUDAとDiffSinger、WORLDLINE-Rはruntimeを含むプロファイルでだけ配布します。日本語軽量版では対応するmanifestも除外します。CUDAはリリース版へ含めません。

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

GUIではbackendが`utau-external-resampler`のRendererを選択した場合だけ、ResamplerとWavtoolの欄を表示します。したがって`classic-utau`以外の公開IDでもClassic UTAUを利用できます。外部wavtoolを使わない場合は`builtin`を選びます。配置後は「Classic UTAUツールを再読み込み」を選びます。

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

## 配布物

release buildは`renderer/`と`models/`をGUI・Serverへコピーします。モデルが一つもない場合はビルドに失敗します。各モデル、Renderer、外部assetのライセンスは[ライセンス表示](../THIRD_PARTY_NOTICES.txt)と配布元の文書を確認してください。
