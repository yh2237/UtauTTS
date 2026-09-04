# 構成

UtauTTSは、UTAUボイスバンクの原音を選び、時間と音高を整えて接続する連結型TTSです。

## 合成の流れ

```text
文章
  ↓ 読み・モーラ・アクセント
原音候補（oto.ini / prefix.map）
  ↓ フレーズ全体で候補を選択
合成計画（時刻、長さ、音高）
  ↓ Renderer
WAV / LAB
```

1. `frontend`が文章を読みとモーラへ変換します。必要に応じてOpen JTalk frontendからアクセント句や品詞などを受け取ります。
2. `voicebank`が`oto.ini`、`prefix.map`、subbankから原音候補を作り、隣り合う原音のつながりも考えてフレーズ全体の経路を選びます。
3. `prosody`がモーラ長、音量、ピッチを決めます。GUIで手動編集した値は自動予測より優先されます。
4. `render`が合成計画に従って原音を配置し、Rendererごとの方法でWAVへ変換します。

原音選択とRendererの間は`internal/plan`の合成計画でつながっています。同じ計画を複数のRendererへ渡せるため、原音選択の差と波形処理の差を分けて比較できます。

## Renderer

Renderer pluginのID、表示名、必要なファイルは`plugins/renderers/*/plugin.json`で管理します。`classic-utau`だけは内蔵定義です。追加方法は[モデル／Rendererプラグイン](plugins.md)、内部処理は[技術設計ガイド](technical-design.md)を参照してください。

| ID | 概要 |
| --- | --- |
| `utautts-world-phrase` | 既定。原音ごとのWORLD特徴を共通の時間軸へ配置し、フレーズ全体を合成 |
| `openutau-worldline-r-faithful` | OpenUTAUのWORLDLINE-R系PhraseSynthを使ってフレーズ全体を合成 |
| `waveform` | Go内で原音波形を伸縮・クロスフェードする比較用Renderer |
| `classic-utau` | 選択したUTAU互換resamplerを実行し、wavtoolまたは内蔵処理で接続 |

未知のRenderer IDはカタログの既定Rendererへ解決されます。assetが不足しているRendererを明示的に選んだ場合はエラーになります。

## 共通の入口

GUI、CLI、HTTP Serverは別々の音声処理を持たず、同じ`synth.Service`を使います。モデル、Renderer、辞書、LAB書き出しも共通です。

- GUI: Qt Quick/QMLからGo backendを呼び出します。
- CLI: `utautts-cli`で一つのWAVを作ります。[CLI](cli.md)
- HTTP Server: `/api/*`から解析、合成、一覧取得を行います。[Server](server.md)
