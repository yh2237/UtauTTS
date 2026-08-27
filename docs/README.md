# UtauTTS ドキュメント

全体像と基本操作はrootの[README](../README.md)にまとめています。このディレクトリには、設定値やAPI、内部実装などの詳細があります。

## 導入と基本操作

- [インストール](installation.md): パッケージの選択、Windows／Linuxでの起動、ボイスバンクの追加
- [GUIの使い方](gui.md): 文章入力、再生、保存、ショートカット、動画編集ソフトとの連携
- [設定](settings.md): 音声合成の初期値、TXT／LAB書き出し、表示、ログ、ショートカット
- [トラブルシューティング](troubleshooting.md): 起動、runtime、Open JTalk、Renderer、Linuxフォントの問題
- [ボイスバンクの利用条件](voicebank.md): 同梱音源の出所、利用条件、ハッシュ

## 機能リファレンス

- [イントネーションとモーラ長の編集](manual-pitch.md): GUI・CLIでの韻律編集とJSON形式
- [辞書設定](dictionary.md): 表記ごとの読みの登録と適用ルール
- [コマンドライン（CLI）](cli.md): CLIでの合成とオプション一覧
- [UtauTTS Server](server.md): HTTPサーバーの起動、API、認証、入力制限

## モデルと拡張

- [モデル／Rendererプラグイン](plugins.md): 検出、互換性、追加方法
- [手動調整から抑揚モデルを作る](prosody-model-training.md): 教師データの収集、監査、学習、評価、追加

## 仕組みを知る

- [UtauTTSはどうやって音声を作るのか](how-utautts-speaks.md): 原音の選択、接続、長さ、イントネーションを図と式を交えて説明
- [構成](architecture.md): 合成パイプラインと配布Rendererの概要
- [技術設計ガイド](technical-design.md): 内部データ、アルゴリズム、Renderer、モデルの実装

## 開発者向け

- [開発環境とビルド](building.md): Windows／Linux版のビルド要件と実行方法
- [リリーステスト](release-testing.md): 配布物の自動スモークテストと手動確認項目
