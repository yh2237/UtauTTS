# 構成

UtauTTSはUTAUボイスバンクに収録された原音を選んで配置・接続し、必要に応じて学習した日本語イントネーションを加えます。ボイスバンク自体や話者の声をニューラルネットワークで生成する方式ではありません。

内部データ構造、各Rendererの処理、研究結果から決めたことは[技術設計ガイド](technical-design.md)に書いてあります。

## 合成パイプライン

1. `frontend`: 入力文章を読みとモーラへ変換します。イントネーションモデルを使う場合は配布物内のOpenJTalk frontendがアクセント句、アクセント核、単語境界、品詞を作ります。
2. `voicebank`: `oto.ini`と`prefix.map`から各モーラの原音候補を作ります。原音設定との合い方を表すtarget scoreと隣接原音の境界を表すjoin scoreを使い、フレーズ全体の経路を選びます。
3. `prosody`: 固定値か自己記述モデルからモーラ長・音量・ピッチを決めます。手動指定がある値はモデルの予測より優先されます。
4. `render`: 合成計画に従って原音を時間配置して波形を作ります。

## 原音選択

VCV、CVVC、CVなどの候補をモーラごとに作り、target scoreと隣接原音のjoin scoreを使うViterbi探索でフレーズ全体の経路を選びます。既定の原音形式`auto`は音源のVC／VCV収録比から選択方針を切り替えます。

## Renderer

RendererのID、表示名、説明、対応機能、必要な資産は`plugins/renderers/*/plugin.json`で管理します。形式と追加方法は[モデル／Rendererプラグイン](plugins.md)にあります。

### `waveform`

Go内で原音を時間伸縮し、クロスフェードで接続します。外部音声処理へ依存しないので原音の明瞭度を確認する比較基準にも使えます。

### `openutau-worldline-r-faithful`

GUI、CLI、Serverの既定Rendererです。OpenUTAU 0.1.565の`PhraseSynth` APIを使い各原音のWORLD特徴を共通の時間軸へ配置してからフレーズ全体を合成します。`worldline.dll`と専用bridgeが必要です。

### `openutau-classic-worldline-faithful`

OpenUTAU Classicに近いphoneme timing、Worldline resampling、5点envelopeを使ってframe modelの相対ピッチ曲線を原音へ適用します。`worldline.dll`と専用bridgeが必要です。

### `openutau-classic-worldline-faithful-gpu`

CPU版faithful Rendererと同じ処理をCUDAで実行する任意Rendererです。CUDA対応の配布物でのみ利用できます。利用できない環境ではCPU版を選択してください。

## GUIとHTTP Server

デスクトップGUIはQt Quick/QMLで作りGoバックエンドをC ABI経由で同一プロセスから呼び出します。音源列挙、読み解析、合成にHTTP ServerやWebViewは使いません。

HTTP Serverは`cmd/utautts-server`と`internal/api`に分かれていてCLIと同じRenderer・モデルcatalogを使います。詳細は[UtauTTS Server](server.md)にあります。
