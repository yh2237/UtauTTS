# UtauTTS ドキュメント

UtauTTSの利用方法と開発資料を目的別にまとめています。利用者向けの入口は[インストール](installation.md)と[GUIの使い方](gui.md)です。

## 利用者向け

- [インストール](installation.md): 対応パッケージ、起動方法、更新、音源の追加
- [GUIの使い方](gui.md): 文章入力、合成、基本編集・拡張編集、保存、AviUtl連携
- [設定](settings.md): 新しい文章の初期値、Renderer設定、辞書、書き出し、表示、ショートカット
- [日本語・英語・中国語の読み上げ](multilingual.md): 言語、読み、発音形式の指定
- [コマンドライン](cli.md): CLIの使い方、手動ピッチ、USTX変換
- [UtauTTS Server](server.md): HTTP API、手動ピッチ編集JSON、サーバーの起動
- [トラブルシューティング](troubleshooting.md): 起動、解析、音声合成の問題
- [同梱音源](voicebank.md): 同梱音源の出所と利用条件

## 仕組み

- [音声合成の仕組み](how-utautts-speaks.md): 原音の選択、接続、長さ、イントネーション
- [技術設計ガイド](technical-design.md): 全体構成、処理の段階と補正、内部データ、実装上の制約

## 開発者向け

- [開発環境とビルド](building.md): Windows、Linux、macOS版の作成
- [Qt WebAssembly版の構成](wasm-architecture.md): Worker、状態管理、仮想ファイルシステム
- [モデル／Rendererプラグイン](plugins.md): Renderer、Classic UTAUツール、モデルの追加
- [リリーステスト](release-testing.md): 配布物の自動検査と手動確認
- [読み上げ品質の評価](../tools/evaluation/README.md): 読み、原音候補、合成音声の比較
- [原音区間ライブラリ](source-understanding.md): 英語・中国語の原音区間の解析、整列、ライブラリ作成
- [モデルの学習](model-training.md): 抑揚・時間伸縮・接続モデルの学習と評価、Intonation Lab
- [DiffSinger](diffsinger.md): DiffSinger連携の対応範囲
- [第三者コードの出典](third-party-provenance.md): 互換処理で参照した公開実装
- [ライセンスの適用範囲](../LICENSE-SCOPE.md): 本体、モデル、音源、第三者コンポーネントの条件
