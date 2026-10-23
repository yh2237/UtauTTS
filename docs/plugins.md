# モデル／Rendererプラグイン仕様

合成エンジン（Renderer）と抑揚モデルは、プラグインとして追加できます。

- モデルの配置先: `models/`
- Rendererの配置先: `renderer/<id>/`

CLIとServerでは、`--model-dir` と `--renderer-dir` で配置先のディレクトリを追加できます。

---

## 1. Renderer Manifest（renderer.json）

Rendererは `renderer/<id>/renderer.json` で定義します。受け付けるスキーマは `manifest_version: 2` です。

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

- `id`: プロジェクトやAPIで使う一意なID。
- `contract`: 入出力の形式（`unit-renderer` または `neural-synthesizer`）。
- `provider`: 実装（`utautts-world-phrase`）。
- `resources`: 必要なバイナリやライブラリの相対パス。OSごとに切り替えるときは `platform_resources` を定義します。

### 外部Providerプロトコル（プロセス間連携）
`protocol: "utautts-provider"` を指定すると、独自の合成エンジンを外部プロセスとして起動し、標準入出力のNDJSONでやり取りします。Providerはセッションモードに対応している必要があります。起動したプロセスは常駐させ、続く合成でも使います。

---

## 2. 抑揚モデル（Prosody Models）

学習済みモデルは `models/` にJSONファイルとして置きます。モデルのJSONがmanifestを兼ねます。

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

- `id` と `display_name` があるJSONをモデルとして読み込みます。
- CLIの `--prosody` にはファイルパスではなく `id` を指定します。
