# モデル／Rendererプラグイン

モデルとRendererは、安定したIDを使ってGUI、CLI、Serverから共通に選びます。配布物では実行ファイルの隣にある`models/`と`plugins/renderers/`を自動検出し、追加の場所は設定または`--model-dir`／`--renderer-dir`で指定できます。

## Renderer

Rendererは`plugins/renderers/<plugin>/plugin.json`で定義します。

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

認識するasset keyは`worldline`、`worldline_bridge`、`world_engine`、`resampler`です。assetのpathはmanifestからの相対pathで指定します。任意のnative codeをUtauTTSへ動的ロードする仕組みはありません。

配布物には次のRendererが入っています。

- `utautts-world-phrase`: 公式WORLDとUtauTTS独自の特徴配置を使う既定Renderer
- `openutau-worldline-r-faithful`: OpenUTAUのWORLDLINE-R系PhraseSynthを使うRenderer
- `waveform`: Go内で原音を伸縮・接続する比較用Renderer

開発用には`utautts-world-phrase-cuda`もありますが、実験的なため配布ZIPには含めません。

未知のIDはカタログの既定Rendererへ解決されます。明示したRendererのassetが不足している場合は、別Rendererへ黙って切り替えずエラーになります。

### 外部UTAU Renderer

GUIの設定でUTAU互換resamplerの実行ファイルを追加すると、`plugins/renderers/<id>/plugin.json`へ次のmanifestが保存されます。実行ファイルは移動せず絶対pathで参照します。

```json
{
  "manifest_version": 1,
  "kind": "renderer",
  "id": "utau-external-example-0123456789ab",
  "display_name": "example",
  "backend": "utau-external-resampler",
  "version": "1",
  "acceleration": "cpu",
  "capabilities": { "frame_pitch": true },
  "assets": { "resampler": "C:/path/to/example.exe" }
}
```

UTAU互換の引数で各原音を処理し、返されたWAVをUtauTTSが共通の時刻と5点包絡線で接続します。flagsや外部wavtoolには対応していません。外部実行ファイルの利用条件は配布元の規約に従ってください。

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
