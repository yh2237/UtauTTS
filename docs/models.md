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

## F0ヘッドを持つモデル

`frame-intonation-tcn-v12`のように、基準モデルにF0ヘッドを重ねるモデルは、`base_model_data`（または`base_model`）と`f0_head`を持ちます。

```json
{
  "id": "my-f0-model",
  "display_name": "My F0 Model",
  "license": "MIT License",
  "license_notices": ["licenses/MY-CORPUS-NOTICE.txt"],
  "provenance": {
    "training_corpus": "Corpus details..."
  },
  "base_model_data": { "...": "基準モデルのJSON" },
  "f0_head": "safetensorsをbase64にしたもの",
  "use_energy": true,
  "base_blend": 0,
  "pitch_offset_cents": -50
}
```

- `base_model_data`は基準モデルのJSONをそのまま埋め込みます。`base_model`に同じフォルダのJSONのパスを書くこともできます。
- `f0_head`はF0（とエネルギー）ヘッドを持つsafetensorsをbase64にしたものです。
- `use_energy`はエネルギーヘッドでモーラの音量を変えるか（未指定は使う）。
- `base_blend`は基準モデルの抑揚曲線を混ぜる重み（0〜1）。
- `pitch_offset_cents`はF0ヘッドの曲線全体へ足す高さ（-1200〜1200セント）。
