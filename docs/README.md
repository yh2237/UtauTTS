# UtauTTS ドキュメント

UtauTTSの利用方法と開発資料を目的別にまとめています。利用者向けの入口: [インストール](installation.md)、[GUIの使い方](gui.md)。

## 利用者向け

- [インストール](installation.md): 対応パッケージ、起動方法、更新、音源の追加
- [GUIの使い方](gui.md): 文章入力、合成、編集、保存、AviUtl連携
- [設定](settings.md): 新しい文章の初期値、書き出し、表示、ショートカット
- [辞書設定](dictionary.md): 表記と読みの登録
- [日本語・英語・中国語の読み上げ](multilingual.md): 言語、読み、発音形式の指定
- [イントネーションとモーラ長の編集](manual-pitch.md): GUIとCLIでの手動調整
- [発話タイミング補正](speech-quality-experiment.md): 音源に合わせたタイミング補正
- [コマンドライン](cli.md): CLIの使い方とUSTX変換
- [UtauTTS Server](server.md): HTTP APIとサーバーの起動
- [トラブルシューティング](troubleshooting.md): 起動、解析、音声合成の問題
- [同梱ボイスバンク](voicebank.md): 同梱音源の出所と利用条件

## 仕組み

- [音声合成の仕組み](how-utautts-speaks.md): 原音の選択、接続、長さ、イントネーション
- [構成](architecture.md): 合成処理の流れと各インターフェース

## 開発者向け

- [開発環境とビルド](building.md): Windows、Linux、macOS版の作成
- [モデル／Rendererプラグイン](plugins.md): Renderer、Classic UTAUツール、モデルの追加
- [技術設計ガイド](technical-design.md): 内部データ、アルゴリズム、実装上の制約
- [リリーステスト](release-testing.md): 配布物の自動検査と手動確認
- [読み上げ品質の評価](../tools/evaluation/README.md): 読み、原音候補、合成音声の比較
- [接続品質の監査と学習](join-quality.md): 接続境界の診断と任意のモデル学習
- [フレーム抑揚モデルの学習](frame-intonation-training.md): 抑揚モデルの学習と評価
- [JSUT音素データと接続事前分布](jsut-target-prior.md): 学習用データの作成と評価
- [DiffSinger](diffsinger.md): DiffSinger連携の対応範囲
- [第三者コードの出典](third-party-provenance.md): 互換処理で参照した公開実装
- [ライセンスの適用範囲](../LICENSE-SCOPE.md): 本体、モデル、音源、第三者コンポーネントの条件
