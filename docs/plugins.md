# モデル／Rendererプラグイン仕様

UtauTTSでは、合成エンジン（Renderer）や抑揚モデルをプラグインとして追加・拡張できます。

- **モデル配置先**: `models/`
- **Renderer配置先**: `renderer/<id>/`

CLIやServerでは `--model-dir` / `--renderer-dir` 引数で外部ディレクトリを追加指定可能です。

---

## 1. Renderer Manifest（renderer.json）

各Rendererは `renderer/<id>/renderer.json` に設定を記述します。現在有効なスキーマは `manifest_version: 2` です。

```json
{
  "manifest_version": 2,
  "kind": "synthesis-engine",
  "id": "example.world",
  "display_name": "Example WORLD Engine",
  "contract": "unit-renderer",
  "provider": "utautts-world-phrase",
  "provider_version": "1",
  "resources": {
    "world_engine": { 
      "path": "../../runtime/engine.dll", 
      "required": true 
    }
  }
}
```

- **id**: プロジェクトやAPIで識別子として使う一意なID。
- **contract**: 入出力インターフェース（`unit-renderer` または `neural-synthesizer`）。
- **provider**: 内部実装エンジン（`utautts-world-phrase`、`utau-external-resampler`、`diffsinger` 等）。
- **resources**: 必要なバイナリやライブラリの相対パス。プラットフォームごとに切り替える場合は `platform_resources` を定義します。

### 外部Providerプロトコル（プロセス間連携）
`protocol: "utautts-provider"` を指定することで、独自の合成エンジンを外部プロセスとして起動し、標準入出力（NDJSON形式）経由で連携させることも可能です。セッション維持に対応したProviderであれば、プロセスを常駐させてモデル初期化オーバーヘッドを回避できます。

---

## 2. Classic UTAUツール（Resampler / Wavtool）

従来のUTAU向けエンジンは、専用ディレクトリにバイナリを配置するだけで利用できます。マニフェストファイルは不要です。

- **Resampler**: `Resamplers/`（サブディレクトリ内の配置や依存DLLの同居にも対応）
- **Wavtool**: `Wavtools/`（内蔵の5点包絡線処理を使う場合は `builtin` を指定）

GUIでProviderが `utau-external-resampler` のRendererを選択すると、使用するResamplerとWavtoolを一覧から切り替えられます。

---

## 3. 抑揚モデル（Prosody Models）

学習済みモデルは `models/` にJSONファイルとして配置します。モデルJSON自体がマニフェストを兼ねています。

```json
{
  "id": "my-model-v1",
  "display_name": "My Custom Model",
  "license": "MIT License",
  "license_notices": ["licenses/MY-CORPUS-NOTICE.txt"],
  "provenance": {
    "training_corpus": "Corpus details..."
  },
  "recommended_renderers": ["utautts-world-phrase"],
  "default_priority": 100,
  "version": 8,
  "feature_version": 1,
  "mode": "intonation_frame_tcn_accent_bounded"
}
```

- `id` と `display_name` が定義されているJSONが自動認識されます。
- CLIではファイルパスではなく、定義した `id` を `--prosody` オプションに指定します。
