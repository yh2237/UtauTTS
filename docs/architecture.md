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

1. `frontend`が言語とphonemizerに応じて文章または読みを音素・モーラへ変換します。日本語の抑揚モデルを使う場合は、必要に応じてOpen JTalk frontendからアクセント句や品詞などを受け取ります。
2. `voicebank`が`oto.ini`、`prefix.map`、subbankから原音候補を作り、隣り合う原音のつながりも考えてフレーズ全体の経路を選びます。
3. `prosody`がモーラ長、音量、ピッチを決めます。GUIで手動編集した値は自動予測より優先されます。
4. `render`が合成計画に従って原音を配置し、Rendererごとの方法でWAVへ変換します。

原音選択とRendererの間は`internal/plan`の合成計画でつながっています。同じ計画を複数のRendererへ渡せるため、原音選択の差と波形処理の差を分けて比較できます。

## Renderer

RendererのID、機能、ランタイムは`renderer/<id>/renderer.json`で定義します。同梱とユーザー定義は同じ探索処理を使い、明示した探索先を優先します。JSONだけで任意の新規engine ABIを追加する機能はありません。追加方法は[モデル／Rendererプラグイン](plugins.md)、内部処理は[技術設計ガイド](technical-design.md)を参照してください。

同梱Rendererはmanifest v2で定義します。実行時のcatalogは`renderer.json`のv1とv2を読み込めますが、v1は既存のユーザー定義を維持するための互換形式です。旧`plugins/renderers/*/plugin.json`は通常のcatalog探索では読み込まず、アプリ内アップデーターが更新中に`renderer/<id>/renderer.json`へ移行します。

| ID | 概要 |
| --- | --- |
| `utautts-world-phrase` | 既定。原音ごとのWORLD特徴を共通の時間軸へ配置し、フレーズ全体を合成 |
| `openutau-worldline-r-faithful` | OpenUTAUのWORLDLINE-R系PhraseSynthを使ってフレーズ全体を合成 |
| `waveform` | Go内で原音波形を伸縮・クロスフェードする比較用Renderer |
| `classic-utau` | 選択したUTAU互換resamplerを実行し、wavtoolまたは内蔵処理で接続 |
| `diffsinger` | DiffSinger音源とbridgeを使う試験的なRenderer（Windows x64のFull配布のみ） |

manifestの公開IDはプロジェクトやUIに保存する名前です。catalog解決時には、公開IDから`engine.ResolvedEngine`を作り、`contract`、`provider`、provider version、typed resource、platform、capabilityの整合性と実行可能性を検証します。`provider`は内蔵Provider registryの実装ID、または`utautts-provider` protocolで接続する外部ProviderのIDです。

Rendererは選択計画のコピーを受け取り、音声と`RenderReport`を返します。Renderer由来の実効timingや境界補正をcanonicalな選択Planへ直接書き戻さないため、同じPlanを複数のRendererで比較できます。出力用のPlanが必要な場合だけ、`RenderedPlan()`でReportをコピーへ適用します。

Renderer IDを省略した場合だけカタログの既定Rendererへ解決されます。未知のIDや、assetが不足しているRendererを明示した場合はエラーになります。

## 共通の入口

GUI、CLI、HTTP Serverは別々の音声処理を持たず、同じ`synth.Service`を使います。モデル、Renderer、辞書、LAB書き出しも共通です。

Open JTalk helper、WORLD bridge、DiffSinger bridge、外部Providerは、初回利用時に起動してsessionを再利用します。アプリ終了時や音源・モデルの再読み込み時にはsessionを閉じます。Classic UTAUのresamplerとwavtoolは、現在もunitごとの外部プロセスです。

- GUI: Qt Quick/QMLからGo backendを呼び出します。
- CLI: `utautts-cli`で一つのWAVを作ります。[CLI](cli.md)
- HTTP Server: `/api/*`から解析、合成、一覧取得を行います。[Server](server.md)
