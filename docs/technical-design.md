# UtauTTS 技術設計ガイド

コマンドの使い方は[CLI](cli.md)、外部から見える構成は[構成](architecture.md)、拡張形式は[モデル／Rendererプラグイン](plugins.md)にあります。

## 1. 中心となる考え方

UtauTTSは、UTAU音源に収録された原音を`oto.ini`に基づいて配置、時間伸縮、接続し、必要に応じて学習済みの韻律を加える連結型TTSです。原音の声質を保ちながら、文章の発音順に並べて音声を生成します。

品質は、明瞭度と子音の保持、接続の滑らかさと声質、リズムとイントネーションの順に評価します。新しい処理は、抑揚の改善と発話内容の明瞭度を両立できる場合に採用します。

## 2. システム全体

```text
GUI / CLI / HTTP Server
          │
          ▼
      synth.Service
          │ 共通入力、音源、モデル、公開Renderer IDを解決
          ▼
       Catalog / engine resolver
          │ Public ID → Definition → Contract / Provider / resources
          ▼
    ┌─ Language frontends ── 読み・音素・モーラ
    ├─ Open JTalk helper ── アクセント・単語・品詞特徴
    ├─ Voicebank resolver ─ 候補ラティスと選択経路
    ├─ Prosody model ────── モーラ長・10msピッチ曲線
    └─ Plan builder ─────── 時刻付きの原音unit列
          │
          ▼
       render.Config
    ├─ UnitRenderer ─────── waveform / WORLD / Classic / external Provider
    └─ NeuralSynthesizer ── DiffSinger score + Provider session
          │
          ▼
       PCM + RenderReport
```

GUI、CLI、HTTP Serverは別々の音声処理を持たず、最終的には同じ`synth.Service`と`tts.Synthesize`へ到達します。入口を追加・変更するときは設定の伝播だけを確認し、音声処理を重複実装しないようにします。各パッケージの担当範囲は[構成](architecture.md)にまとめています。

## 3. テキストからモーラまで

### 読みの生成

読みが明示されていない場合は最初にユーザー辞書を長い表記から順に適用してKagomeとIPA辞書で読みへ変換します。数字やラテン文字など内蔵経路で発音を得られないトークンがあればOpen JTalk frontendへフォールバックします。

日本語は`ja-kana`、英語は`en-arpasing`／`en-delta`／`en-vccv`、中国語は`zh-cvvc`のphonemizerへ分岐します。英語と中国語では日本語用のOpen JTalk抑揚モデルを使わず、英語は規則的な強勢・句曲線、中国語はPinyinの声調曲線を使います。入力形式と制約は[多言語TTS](multilingual.md)にまとめています。

かな入力は`frontend.ParseKana`でモーラ列へ分解されます。各モーラが持つのは少なくとも表記、子音、母音、休止かどうかです。促音、撥音、長音、拗音を文字単位ではなく合成単位として扱います。この段階以降の処理はモーラ列を参照します。

### Open JTalk特徴

`frame-intonation-v8`などのモデルは読みだけでは得られない次の特徴を使います。

- アクセント句内の位置と残り長
- アクセント核との位置関係
- 単語の開始と終了
- 品詞と品詞細分類
- 句境界と発話内位置

Go本体は同梱された`utautts-openjtalk-features`を別プロセスとして起動してJSONで解析結果を受け取ります。helperはアプリの稼働中に常駐し、約100MBの辞書を最初の一度だけ読み込みます。PythonやOpen JTalkをGoプロセスへ直接埋め込まないのでGUI、CLI、Serverから同じ実行形式を利用できます。

Kagome側とOpen JTalk側でモーラ分割が一致しない場合は完全一致、母音一致、長音、skipへ異なるコストを与える動的計画法で対応位置を求めます。対応しなかった位置には最も近い特徴を補います。ここを単純な配列indexで結ぶと未知語や長音以降のアクセント特徴がすべてずれます。

## 4. ボイスバンクの読み込みと原音選択

### メタデータ

ボイスバンク読込は複数の`oto.ini`、UTF-8／Shift_JIS、`prefix.map`、`character.yaml`の基本的なsubbankを扱います。toneとcolorからsubbankを決めてそのprefix／suffixをaliasへ適用します。

空のprefixやsuffixも有効な設定です。文字列の前後を一律にtrimすると空接辞を持つ有効な`prefix.map`行を失い、別音階の原音へ誤ってfallbackすることがあります。メタデータ処理では「空として明示された値」と「設定が見つからない状態」を区別します。

### 候補ラティス

モーラごとに次の候補を生成します。

- VCV
- CV
- VC + CVの複合候補
- wildcardや表記差のfallback
- 促音原音がない場合の無音`<closure>`

CVVCのVCは一つのモーラではなく次のCVへ入る`transition` unitです。Plan上では主unitと分けて保持し、モーラ全体の時間は主unitで管理します。

`AliasPolicy=auto`は音源内のVC／VCV収録比を見て標準プロファイルかCVVC向けプロファイルを選びます。CVVC向けプロファイルではCVVC候補を優先してtransitionをsequential timingで置き、VC音量を35%にします。明示指定されたpolicyは自動判定より優先です。

候補WAVは選択前に構造検証されます。存在しないWAV、読めない形式、成立しない切り出し範囲などは候補から外して理由をPlanへ残します。候補数は各位置で最大32件に制限して組合せ爆発を防ぎます。

### 経路選択

標準は発話の休止区間ごとに行うViterbi探索です。概念上の経路スコアは次の和です。

```text
path score = Σ local candidate score + Σ adjacent join score
```

local scoreにはaliasのfallback段階、`oto.ini`値の整合性、subbankや形式の優先度が入ります。

join scoreは隣接原音のenergy、スペクトル、F0などの境界特徴と同じ録音groupかどうかを評価します。同じWAV内の前向きなanchorには加点しますが、560 msを超える離れた移動は加点を減らします。VCVのincoming側が閉鎖区間になる場合は減点を弱め、母音側の候補scoreを優先します。手設計scoreを使うViterbi探索で経路を決めます。

候補が疎なUTAU音源では、境界の連続性を優先すると音素文脈や声質が変わることがあります。そのためjoin scoreは候補の言語的な適合性を置き換えず、保守的な補助値として扱います。

## 5. Planは段階間の契約

`internal/plan`のPlanは原音選択とRendererを分離する中間表現です。各unitには次の情報が入ります。

- 元のモーラ位置とunitの役割
- alias、WAV、`oto.ini`のファイルと行
- 発音開始時刻とモーラ長
- offset、consonant、cutoff、preutterance、overlap
- pitchとenergyの係数
- target、join、累積path score
- fallback段階、subbank、tone、color
- 候補の除外理由と音響score
- Rendererへ渡す前のselection値。Renderer由来の実効値は`RenderReport`へ分離

Planは、候補選択、時間設計、Rendererの差を切り分けるための再現可能な記録です。新しい自動補正では、入力値、実際に使った値、採用理由、フォールバック理由を記録します。最終WAVとPlanを組み合わせることで、品質低下の原因を追跡できます。

共有Planは読み取り専用で扱います。`UnitRenderer`にはPlanのコピーを渡し、Rendererが計算したleading margin、boundary bridge、実効preutterance／consonant／overlap、source／target F0などを`RenderReport`として返します。CLIのPlan JSONやLABのように描画後の値が必要な出力だけが、`tts.Result.RenderedPlan()`でコピーへReportを適用します。

## 6. 韻律モデル

### モデル形式

モデルは任意コードではなく重みとメタデータを持つ自己記述JSONです。`id`、`version`、`feature_version`、`mode`からGo側の決定論的な推論器を選びます。未知形式、壊れたshape、ID重複は読み飛ばさずカタログ構築時のエラーにします。

同梱モデルは次の3つです。

| モデル | 形式 | 出力 |
| --- | --- | --- |
| `frame-intonation-v8` | version 8 / feature 1 | 10ms単位の相対ピッチ |
| `prosody-multitask-v1` | version 10 / feature 2 | v8系ピッチとモーラ長倍率 |
| `english-intonation-v1` | version 12 / feature 1 | 英語の強勢と句末境界の10ms単位ピッチおよび長さ倍率 |

v8のframe headはモーラとOpen JTalk由来特徴をフレームへ展開してdilationを持つ小型TCNで相対pitchを予測します。同梱モデルは406特徴、10ms間隔、学習出力範囲±250 centです。推論後の処理: モデル内のrender strength、平滑化、percentile／最大値制約。学習音声に由来する細かなF0揺れは、この処理で強度を調整します。

multitaskモデルは同じframe headへ423特徴からモーラ長倍率を出すduration headを加えたものです。絶対msではなく基準モーラ長に対する倍率なのでGUIの話速設定や音源差と共存できます。

英語モデルは外部特徴を要求せず、ARPAbetから得た強勢、語境界、句境界を決定論的な軽量ヘッドへ入力します。英語のカードで日本語モデルが選択されている場合は同じフォルダの英語モデルへ切り替えます。

version 11のmanual residual形式もruntimeが解釈できます。これはv8を基準にGUIで行った人手修正の傾向だけを小さなcent補正として学習する形式です。元モデルのSHA-256と補正範囲を持ち、基準モデルへ残差を加えます。標準配布: version 11モデルなし。

### 推論順序

1. モーラ列とOpen JTalk特徴を作ります。
2. モーラ単位のduration、pitch、energy予測を作ります。
3. 手動モーラ長がある位置は自動durationより優先します。
4. 確定した長さからPlanと各モーラの時刻を作ります。
5. その時刻へ10ms frame pitchを生成します。
6. 自動pitchへ手動pitchを加算、または置換します。
7. Rendererの能力と`ApplyPitch`を確認して波形へ適用します。

GUIの解析プレビューも同じ順序を使います。自動値を0などの特殊値で隠さず予測済みの値を画面へ返します。ユーザーが編集した位置だけをoverrideとして保持して再生時の再解析で表示値が変わらないようにしています。

### 相対ピッチ

モデルの出力: 話者の絶対F0ではなく、発話内基準に対するcent値。

```text
cents = 1200 × log2(F0 / reference F0)
```

Rendererは各原音のF0を測ってこの相対曲線を音源側の声域へ重ねます。学習話者の声の高さをボイスバンクへコピーせず抑揚の形だけ借りるわけです。

## 7. Renderer

Renderer manifestの`id`は保存データやUIで使う公開識別子です。catalogはこの公開IDを`engine.ResolvedEngine`へ解決し、`contract`、`provider`、provider version、typed resource、platform、capabilityを検証します。manifestは表示情報とruntime resourceを宣言します。native codeや新しいengine ABIの追加: Go実装。標準Rendererも`renderer/<id>/renderer.json`から読み込みます。`utau-external-resampler` providerは`Resamplers/`と`Wavtools/`の実行ファイルを組み合わせます。

設定の境界もRenderer単位で分けます。`tts.Config`はテキスト、音源、モデル、Plan作成に必要な共通入力と解決済み`engine.ResolvedEngine`を持ちます。Classicの実行ファイルやWORLDの専用スイッチは`render.Config`の`render.ProviderOptions`へ分離し、Classicは`ClassicOptions`、WORLDは`WorldlineProviderOptions`へ固有設定を閉じ込めます。

### waveform

`waveform`はGoだけで動く原波形を確認しやすい基準Rendererです。

1. WAVをmonoへ変換し、発話内のsample rateを統一します。
2. `oto.ini`のoffsetとcutoffで原音を切り出します。
3. pitchが有効なら、固定係数またはframe曲線を可変レートresamplingで適用します。
4. 子音側の長さを保護しながらWSOLAで目標長へ合わせます。
5. `NoteStart - preutterance`の絶対時刻へ配置します。
6. 最低6msを確保した相補的なsmoothstep包絡で受け渡します。
7. 重なりgainを正規化し、最後にpeakだけを安全範囲へ収めます。

発話先頭では負のpreutterance分を捨てず出力全体へleading marginを加えます。これがないと最初の子音が半分ほど失われる音源があります。

`waveform`はframe pitchに対応しますが直接resamplingとWSOLAを組み合わせると曲線や圧縮条件によって周期的な震えやケロケロ感が出ることがあります。

位相を合わせた短い境界bridgeも診断機能として実装していますが標準では無効です。

### Classic UTAU Renderer

`classic-utau`は内部backendの`utau-external-resampler`を使います。OpenUTAU由来のphone timingに従ってresamplerをunitごとに呼び、tone、offset、必要長、consonant、cutoff、12bit形式のpitch列などを渡します。返されたWAVはsample rateを統一し、内蔵処理または選択したwavtoolで接続します。

resamplerとwavtoolは独立したプロセスです。終了コード、出力WAV、timeoutを呼び出しごとに検査します。velocity、flags、volume、modulation、tempoはunit単位でも指定できます。

### UtauTTS WORLD phrase

`utautts-world-phrase`は現在の既定Rendererです。OpenUtauの`PhraseSynth`を使わず、公式WORLDのHarvest、CheapTrick、D4C、SynthesisだけをDSP部品として使います。原音の切り出し、子音を保つ時間写像、前後のfade、特徴量の補間と重なりの処理はUtauTTS側にあります。

原音ごとのF0、スペクトル包絡、非周期性指標はbridge内にキャッシュします。再生時はこれらをフレーズの10 ms時間軸へ置き直し、隣接する分析frameを補間してから一度だけWORLD合成します。

### DiffSinger

`diffsinger`は専用のDiffSinger音源を`dsconfig.yaml`から読み込み、Windows x64のbridgeを通じて音響モデルとvocoderを実行します。通常のUTAU音源の`oto.ini`、resampler、wavtoolを使う経路とは異なります。対応範囲と配置は[DiffSinger](diffsinger.md)を参照してください。

### Rendererを変更するときの境界

Rendererは選択済みunitのaliasを維持します。候補選択の改善: `voicebank`。時間付きunit列の変更: `plan`。波形処理の変更: `render`。この境界により同じPlanを複数Rendererへ渡して比較できます。

内蔵Providerと外部Providerは同じ`UnitRenderer`境界で扱います。内蔵側もcloneしたPlanへ処理を行い、外部側は`utautts-provider`のhandshakeと共通jobを通じて音声と診断を返します。DiffSingerはUTAUのUnit Planを入力にするRendererではなく、`neural-synthesizer` contractの`NeuralScore`経路を使います。

## 8. GUI、CLI、Serverとキャッシュ

Qt GUIはQMLからC ABIで`internal/native.Engine`を呼び出します。音源の列挙、解析、韻律プレビュー、合成、exo出力は、メソッド名とJSONでやり取りします。GUIはHTTPやWebViewを介さず、C ABIで合成処理へ接続します。

CLIとHTTP Serverも同じRendererカタログと`synth.Service`を使います。モデルとRendererはファイル名ではなくIDで選択し、明示したディレクトリ、配布物内のディレクトリ、開発用ワークスペースの順に解決します。必要なファイルの相対パスは、Rendererディレクトリを基準に絶対パスへ変換します。

反復編集を軽くするため次をプロセス内でcacheします。

- 読込済みボイスバンク
- サイズと更新時刻で検証したモデル
- 最大512件のOpen JTalk解析
- 最大256MiBのデコード済みWAV LRU

音源再読込ではこれらを明示的に破棄します。新しいcacheを追加するときはファイル更新の検知、上限、明示clear、並行アクセスの4点を揃えてください。

外部helperとProviderの寿命はcacheとは別に管理します。Open JTalk helper、WORLD bridge、DiffSinger bridge、session対応の外部Providerは初回利用時にlazy startし、同じEngine instanceで複数の合成要求から再利用します。通常の「1合成1プロセス」にはせず、cancel／timeoutやprocess終了時だけ再起動します。Classic UTAUのresamplerとwavtoolは現状ではunitごとのprocessです。

合成要求は`context.Context`でキャンセルされます。外部helperにはtimeoutと終了待ち時間があり長いunit処理中も定期的にキャンセルを確認します。GUIだけでなくServerの切断や終了処理でも同じ仕組みを使います。

## 9. 設計上の制約

### `oto.ini`の扱い

`oto.ini`は原音の切り出し、子音固定部、先行発声、overlapを記述する基礎情報です。Rendererはこれを捨てて波形だけから境界を推定することはなく、必要な補正も元の定義から大きく外れない範囲に制限します。

原音をTTS向けの短い長さへ強く圧縮すると震え、途切れ、子音欠落が生じます。固定msだけでなく原音長に対する圧縮率と子音部の保護も考える必要があります。

### 連続anchorは高品質でも適用範囲が狭い

同じ録音WAV内の隣接原音は候補選択で連続性を加味します。原音を分割せず連続した時間写像へ置き換える方式は対応する録音列と境界管理が必要なため、標準Rendererでは採用条件を限定します。

### 波形補修と候補選択には上限がある

波形補修の入力: 波形に残っている情報。候補選択の対象: 収録済みの音素文脈。補修やscoreの追加条件: 候補密度、原音の文脈、適用位置。

## 10. 変更時の方針

### 公開IDと既存経路を維持する

既存Rendererの互換性: ID、意味、出力を維持します。新方式は別のRenderer IDまたは初期無効の明示オプションとして追加します。評価が不十分な段階でも、同じ入力で標準の出力を再現できる状態を保ちます。

### fallbackの扱いを統一する

新しい制御値には範囲制限を設けます。入力検証のエラー条件: NaN、非単調なtime anchor、過大なcrop、必要なファイルの不足。Renderer IDを省略した場合はカタログの既定Rendererへ解決し、未知の明示IDはエラーにします。低信頼度のunitや境界だけをfallbackする場合は、その位置と理由をPlanへ残します。
