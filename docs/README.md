# UtauTTS ドキュメント

まずはプロジェクトルートの[README](../README.md)を読んでください。ここでは、READMEから参照している詳細資料を目的ごとにまとめています。

## 使い方

- [インストール](installation.md): パッケージ、起動、音源の追加
- [GUIの使い方](gui.md): 文章入力、編集、再生、保存、exo出力
- [設定](settings.md): 初期値、書き出し、表示、ログ、ショートカット
- [辞書設定](dictionary.md): 表記と読みの登録
- [コマンドライン](cli.md): CLIのオプションとUSTX変換
- [UtauTTS Server](server.md): HTTP API、認証、入力制限
- [トラブルシューティング](troubleshooting.md): 起動・解析・合成の問題

## 仕組みと拡張

- [UtauTTSはどうやって音声を作るのか](how-utautts-speaks.md): 原音、接続、長さ、イントネーションの説明
- [構成](architecture.md): 合成パイプラインと各入口の関係
- [技術設計ガイド](technical-design.md): 内部データ、アルゴリズム、実装上の制約
- [モデル／Rendererプラグイン](plugins.md): Renderer、Classic UTAUツール、モデルの追加
- [DiffSinger](diffsinger.md): 試験実装の対応範囲と音源配置
- [イントネーションとモーラ長の編集](manual-pitch.md): GUIとJSON／CLI
- [手動調整から抑揚モデルを作る](prosody-model-training.md): 教師データの収集と学習

## 開発と配布

- [開発環境とビルド](building.md): Windows／Linux版のビルド
- [リリーステスト](release-testing.md): 配布物の自動検査と手動確認
- [同梱ボイスバンク](voicebank.md): 出所、利用条件、ハッシュ
