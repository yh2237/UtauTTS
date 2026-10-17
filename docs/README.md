# UtauTTS ドキュメント

## はじめに
- [インストールと起動](installation.md) - OSごとの手順と必要な環境
- [GUIの使い方](gui.md) - 編集、書き出し
- [設定リファレンス](settings.md) - 合成の初期値、Renderer設定、辞書設定、書き出し

## 機能別ガイド
- [日本語・英語・中国語の読み上げ](multilingual.md) - 多言語音源の使い方と発音形式
- [CLI（コマンドライン）リファレンス](cli.md) - コマンドラインでの合成と一括処理
- [HTTP APIサーバー仕様](server.md) - REST APIのエンドポイントと連携の仕様
- [DiffSinger](diffsinger.md) - ニューラル音源の利用
- [トラブルシューティング](troubleshooting.md) - よくある不具合と対処
- [同梱音源について](voicebank.md) - 初期音源の出典と利用条件

## 仕組みと開発者向け資料
- [音声合成の仕組み](how-utautts-speaks.md) - 原音の選択、抑揚の予測、波形の接続
- [技術設計ガイド](technical-design.md) - システムの構成とデータ構造
- [モデル／Rendererプラグイン](plugins.md) - 合成エンジンと抑揚モデルの拡張仕様
- [モデルの学習手順](model-training.md) - 独自の抑揚モデルの学習
- [開発環境とビルド](building.md) - ソースコードからのビルドとテスト
- [Qt WebAssembly版の構成](wasm-architecture.md) - Web版の非同期通信と仮想ファイルシステム
- [リリーステスト](release-testing.md) - 配布パッケージの作成と検査、更新の互換性の確認
- [第三者コードの出典](third-party-provenance.md) - 互換処理で参照した公開実装
- [ライセンスの適用範囲](../LICENSE-SCOPE.md) - 成果物とライセンスの対応表
