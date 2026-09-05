# モデル／Rendererプラグイン

モデルとRendererは、安定したIDを使ってGUI、CLI、Serverから共通に選びます。実行ファイルの隣にある`models/`と`plugins/renderers/`を自動検出します。CLIとServerでは`--model-dir`／`--renderer-dir`で探索先を追加できます。

## Renderer

Rendererは音声合成方式全体を表します。`UtauTTS WORLD phrase`、`OpenUTAU WORLDLINE-R faithful`、`Waveform`、`Classic UTAU`が該当します。Classic UTAUだけがresamplerとwavtoolを組み合わせます。

## Classic UTAUツール

resamplerは`Resamplers/`、wavtoolは`Wavtools/`へ配置します。`plugin.json`は不要です。OpenUtauと同様にサブディレクトリも探索するため、依存DLLを実行ファイルと同じディレクトリへ置けます。プロジェクトには絶対パスではなく、各ディレクトリからの相対IDを保存します。

```text
Resamplers/
  moresampler.exe
  L2R/
    L2R.exe
    dependency.dll
Wavtools/
  wavtool.exe
```

GUIではRendererに`Classic UTAU`を選択した場合だけ、ResamplerとWavtoolの選択欄が表示されます。外部wavtoolを使わない場合は`UtauTTS built-in`を選びます。

各フォルダは「ファイル」メニューから開けます。配置後は「Classic UTAUツールを再読み込み」を選びます。

### Rendererパッケージ v2

同梱RendererはGo側のレジストリで定義します。内蔵Rendererを使うだけなら`plugin.json`の編集は不要です。外部パッケージだけが`plugins/renderers/<plugin>/plugin.json`を持ちます。Classic UTAUのresamplerやwavtoolとは別の仕組みです。

GUIの「ファイル」→「Rendererプラグイン」からZIPを導入できます。ZIPにはv2の`plugin.json`を一つだけ含めます（トップレベルのフォルダで包んでも構いません）。検証後に配置して一覧を再読み込みします。既存IDは上書きしません。更新する場合は既存フォルダを探索先の外へバックアップしてから導入してください。更新前後でIDは変えないでください。

最小例は次のとおりです。機能情報と標準ランタイムの場所はbackendから継承するので、重複して書く必要はありません。

```json
{
  "manifest_version": 2,
  "kind": "renderer",
  "id": "example.renderer",
  "display_name": "Example Renderer",
  "description": "画面に表示する説明",
  "backend": "waveform",
  "version": "1.0.0",
  "protocol_version": 1
}
```

`id`はプロジェクトやAPIへ保存する公開ID、`backend`はUtauTTS内の実装を選ぶIDです。新しい合成方式をJSONだけで実装できる仕組みではありません。内蔵IDは予約されています。壊れたmanifestや未対応backendは診断としてGUIとAPIの`problems`へ表示し、アプリ全体の起動を妨げません。`default_priority`が最大のRendererがカタログの既定値です。

詳しい形式は[JSON Schema](renderer-plugin-v2.schema.json)を参照してください。`runtimes`ではasset keyごとに同梱ランタイムの`id`と契約の`version: "1"`を指定できます。通常は省略して標準を使います。asset keyはbackendに応じて`worldline`、`worldline_bridge`、`world_engine`、`world_gpu`、`diffsinger_bridge`です。

独自バイナリを同梱する場合は`platforms.windows-amd64`や`platforms.linux-amd64`（共通なら`any`）の下に、asset keyと`{ "path": "bin/engine.dll", "sha256": "64桁のハッシュ" }`を指定します。pathはパッケージ内の相対パスに限定し、利用するプラットフォームのファイルをSHA-256で検証します。`any`よりOS・CPU別の指定が優先されます。ハッシュは改ざんの署名検証ではありません。実行ファイルやDLLを含むため、信頼できる配布元のZIPだけを導入してください。

ZIP展開は一時ディレクトリで行い、パス逸脱、シンボリックリンク、重複パス、4096件を超えるエントリ、展開後512 MiB超を拒否します。v2の未知フィールドはエラーにします。従来のv1形式は手動配置で引き続き読み込めます。同梱Rendererと同じID・backendの旧v1定義は内蔵定義に置き換えて扱います。

配布物には次のRendererが入っています。

- `utautts-world-phrase`: 公式WORLDとUtauTTS独自の特徴配置を使う既定Renderer
- `openutau-worldline-r-faithful`: OpenUTAUのWORLDLINE-R系PhraseSynthを使うRenderer
- `waveform`: Go内で原音を伸縮・接続する比較用Renderer
- `classic-utau`: 配置したresamplerとwavtoolを使うRenderer

開発用には`utautts-world-phrase-cuda`もありますが、実験的なため配布ZIPには含めません。

IDを省略した場合だけ既定Rendererへ解決します。未知のIDや、明示したRendererのasset不足はエラーにし、別Rendererへ黙って切り替えません。

### Classic UTAU互換仕様

UTAU互換の引数で各原音を処理します。外部wavtoolを選んだ場合はOpenUtau Classic Rendererと同じ位置引数で接続し、`builtin`ではUtauTTS内蔵の5点包絡線処理を使います。外部実行ファイルの利用条件は配布元の規約に従ってください。

resamplerの呼び出し形式はOpenUtau Classic Rendererと同じ13引数です。入力WAV、出力WAV、音高、velocity、flags、offset、必要長、consonant、cutoff、volume、modulation、tempo、12bit Base64ピッチ列の順に渡します。既定値はvelocity 100、空のflags、modulation 0、tempo 120です。

ノート単位の設定はAPIの`resampler_expressions`、またはCLIの`--resampler-expressions`で指定します。`position`は読みのモーラ位置で、CVVC transitionにも親モーラの設定が引き継がれます。省略した値はRenderer全体の設定を使います。

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
  "description": "モデルの用途と学習条件",
  "recommended_renderers": ["utautts-world-phrase"],
  "default_priority": 100,
  "version": 8,
  "feature_version": 1,
  "mode": "intonation_frame_tcn_accent_bounded"
}
```

`id`と`display_name`がないJSONはモデルとして扱いません。同じIDを複数置くと診断に表示されます。CLIの`--prosody`にはファイルpathではなくIDを指定します。

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

release buildは`plugins/renderers/`と`models/`の内容をGUI・Serverへコピーします。モデルが一つもない場合はビルドに失敗します。各モデル、Renderer、外部assetのライセンスは[ライセンス表示](../THIRD_PARTY_NOTICES.txt)と配布元の文書を確認してください。
