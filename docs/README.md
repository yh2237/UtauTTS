# UtauTTS ドキュメント

UtauTTSの利用方法および技術ドキュメントの目次です。

## はじめに
- [インストールと起動](installation.md) - 各OSでのセットアップ手順・必要環境
- [GUIの使い方](gui.md) - 基本的な編集から音声書き出し、AviUtl連携まで
- [設定リファレンス](settings.md) - 合成初期値、Renderer詳細設定、辞書設定、ショートカット

## 機能別ガイド
- [日本語・英語・中国語の読み上げ](multilingual.md) - 多言語音源での利用方法と発音形式
- [CLI（コマンドライン）リファレンス](cli.md) - コマンドラインからの音声合成・一括処理
- [HTTP APIサーバー仕様](server.md) - REST APIエンドポイントと連携仕様
- [DiffSinger](diffsinger.md) - ニューラル音源の利用
- [トラブルシューティング](troubleshooting.md) - よくある質問と不具合への対処
- [同梱音源について](voicebank.md) - 初期音源の出典と利用条件

## 仕組みと開発者向け資料
- [音声合成の仕組み](how-utautts-speaks.md) - 原音選択・抑揚予測・波形接続のアルゴリズム
- [技術設計ガイド](technical-design.md) - システムアーキテクチャとデータ構造
- [モデル／Rendererプラグイン](plugins.md) - 合成エンジンや抑揚モデルの拡張仕様
- [モデルの学習手順](model-training.md) - 独自イントネーションモデルの学習方法
- [開発環境とビルド](building.md) - ソースコードからのビルド手順
- [Qt WebAssembly版の構成](wasm-architecture.md) - Web版の非同期通信・仮想ファイルシステム
- [リリーステスト](release-testing.md) - 自動検査パッケージ検証
- [第三者コードの出典](third-party-provenance.md) - 互換処理で参照した公開実装
- [ライセンスの適用範囲](../LICENSE-SCOPE.md) - 成果物とライセンスの対応表
