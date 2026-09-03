# モデル／Rendererプラグイン

モデルとRendererは、安定したIDを使ってGUI、CLI、Serverから共通に選びます。配布物では実行ファイルの隣にある`models/`と`plugins/renderers/`を自動検出し、追加の場所は設定または`--model-dir`／`--renderer-dir`で指定できます。

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

UtauTTS WORLD phraseなどの同梱Rendererは`plugins/renderers/<plugin>/plugin.json`で定義しています。これはRenderer全体の定義であり、Classic UTAUのresamplerやwavtoolには使いません。

```json
{
  "manifest_version": 1,
  "kind": "renderer",
  "id": "example.renderer",
  "display_name": "Example Renderer",
  "description": "画面に表示する説明",
  "backend": "waveform",
  "version": "1",
  "experimental": false,
  "default_priority": 0,
  "capabilities": {
    "frame_pitch": false,
    "boundary_bridge": true
  },
  "assets": {}
}
```

`id`はプロジェクトやAPIへ保存する公開ID、`backend`はUtauTTS内の実装を選ぶIDです。manifestの破損、未対応backend、ID重複は起動時エラーになります。`default_priority`が最大のRendererがカタログの既定値です。

認識するasset keyは`worldline`、`worldline_bridge`、`world_engine`、`resampler`、`wavtool`です。assetのpathはmanifestからの相対pathで指定します。任意のnative codeをUtauTTSへ動的ロードする仕組みはありません。

配布物には次のRendererが入っています。

- `utautts-world-phrase`: 公式WORLDとUtauTTS独自の特徴配置を使う既定Renderer
- `openutau-worldline-r-faithful`: OpenUTAUのWORLDLINE-R系PhraseSynthを使うRenderer
- `waveform`: Go内で原音を伸縮・接続する比較用Renderer

開発用には`utautts-world-phrase-cuda`もありますが、実験的なため配布ZIPには含めません。

未知のIDはカタログの既定Rendererへ解決されます。明示したRendererのassetが不足している場合は、別Rendererへ黙って切り替えずエラーになります。

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

`id`と`display_name`がないJSONはモデルとして扱いません。同じIDを複数置くと起動時エラーになります。CLIの`--prosody`にはファイルpathではなくIDを指定します。

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
