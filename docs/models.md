# 抑揚モデルの追加

学習済みモデルは `models/` にJSONファイルとして置きます。CLIとServerでは、`--model-dir` で探す場所を追加できます。

```json
{
  "id": "my-model-v1",
  "display_name": "My Custom Model",
  "license": "MIT License",
  "license_notices": ["licenses/MY-CORPUS-NOTICE.txt"],
  "provenance": {
    "training_corpus": "Corpus details..."
  },
  "default_priority": 100,
  "version": 8,
  "feature_version": 1,
  "mode": "intonation_frame_tcn_accent_bounded"
}
```

- `id` と `display_name` があるJSONをモデルとして読み込みます。
- CLIの `--prosody` にはファイルパスではなく `id` を指定します。
