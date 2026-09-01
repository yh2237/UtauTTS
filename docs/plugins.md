# モデル／Rendererプラグイン

UtauTTSでは画面や配布scriptにモデル名とRenderer名を直接並べていません。各要素が安定ID、表示名、説明、互換情報を持ち、アプリケーション側は検索directoryと既定IDだけを扱います。

## Renderer

Rendererは`plugins/renderers/<plugin>/plugin.json`を1つ持ちます。

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

`id`は保存データやAPIで使うプラグイン固有ID、`backend`はUtauTTSが実行する実装adapterです。分けてあるので同じbackendへ別の名前、資産、既定値を持つRendererを追加できます。native codeをUtauTTSのプロセスへ直接ロードする仕組みはありません。

壊れたmanifest、未対応backend、IDの重複は起動時エラーになります。

`default_priority`が最大のものがcatalog上の既定Rendererで同値なら`display_name`順です。GUIでは設定に保存したデフォルトRendererを優先し、未設定なら`utautts-world-phrase`を使います。CLI／Serverの`--renderer`はこの値を上書きします。`acceleration`には`cpu`か`cuda`を指定できて必要DLLやモデルはmanifestからの相対pathを`assets`へ記述します。

認識するasset key:

- `worldline`
- `worldline_bridge`
- `world_engine`
- `resampler`: `utau-external-resampler` backendで実行するUTAU互換resampler

配布物に入っているRenderer IDは次のとおりです。

- `waveform`: CPUで動作する標準の波形接続
- `utautts-world-phrase`: 公式WORLDとUtauTTS独自の特徴配置を使う既定Renderer
- `openutau-worldline-r-faithful`: OpenUTAUの処理でフレーズ全体をWORLD合成するRenderer

### 外部UTAU Renderer

GUIの設定で実行ファイルを追加すると、UtauTTS直下の`plugins/renderers/<id>/plugin.json`へ次の形式で保存されます。実行ファイルは移動やコピーをせず、絶対pathで参照します。

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

呼び出し形式はUTAU互換で、入力WAV、出力WAV、音名、子音速度、flags、offset、必要長、consonant、cutoff、音量、modulation、tempo、ピッチ列を渡します。wavtoolは外部から指定せず、UtauTTSがOpenUTAU由来のタイミングと5点包絡線で接続します。resampler固有flagsの設定にはまだ対応していません。

GUI、CLI、Serverは同じ`plugins/renderers`を自動検出します。CLI／Serverでは`--renderer`へmanifestの`id`を渡します。外部実行ファイルの利用条件はUtauTTSのライセンスには含まれないため、各配布元の規約を確認してください。

Qt GUI、native backend、HTTP API、CLIは同じcatalogを使います。追加directoryはnative JSONの`renderer_directories`かCLI／Serverの`--renderer-dir`で指定できます。`--renderer-dir`は繰り返し指定しても大丈夫です。

## 抑揚モデル

モデルには別のmanifestを用意せずモデルJSON自身がidentityを持ちます。モデルIDはGUI、CLI、Serverで共通です。

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

`id`と`display_name`のないJSONはモデルpluginとして扱いません。学習scriptは上のfieldを出力します。過去の学習出力を移行する場合はinstallerへidentityを明示してJSON自体を書き換えます。

```powershell
.\tools\install-prosody-model.ps1 `
  -ModelPath .\out\prosody\my-model.json `
  -Id my-model-v1 `
  -DisplayName "My intonation model" `
  -RecommendedRenderer utautts-world-phrase `
  -DestinationDirectory .\models
```

GUIとServerは`models/`を走査します。追加directoryはnative JSONの`model_directories`かCLI／Serverの`--model-dir`で指定します。CLIの`--prosody`に指定するのはmodel IDです。

GUIで調整した抑揚からversion 11の個人補正モデルを作る手順は[手動調整から抑揚モデルを作る](prosody-model-training.md)にあります。

配布モデルは次の2種類です。

- `frame-intonation-v8`: Open JTalkのアクセント特徴を使ったフレーム単位のイントネーション
- `prosody-multitask-v1`: v8系のイントネーションに加えたモーラ長予測

どちらもfaithful系Rendererを推奨します。

## 配布物

release buildは`plugins/renderers/`と`models/`へinstallされたものをGUI・Serverへコピーします。`models/`が空ならビルドは失敗します。
