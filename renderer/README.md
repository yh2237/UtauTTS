# Renderer定義

Rendererは`renderer/<id>/renderer.json`で定義します。配布時の定義と、`--renderer-dir`で追加するユーザー定義は同じ形式で読み込みます。同じIDがある場合は、明示したディレクトリの定義を優先します。

実行時に使うファイルは、パッケージ直下の`runtime/`へ配置します。manifestのパスはRendererディレクトリからの相対パスです。OSごとにファイルが異なる場合は`platform_resources`を使用します。定義はすべてmanifest version 2です。詳しくは[プラグイン仕様](../docs/plugins.md)と[スキーマ](../docs/renderer.schema.json)を参照してください。

アプリケーションには、同梱manifestが指定するProviderの実装が組み込まれています。manifestだけで任意の合成エンジンを動的に追加することはできません。
