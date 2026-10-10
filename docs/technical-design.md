# UtauTTS 技術設計ガイド

コマンドの使い方は[CLI](cli.md)、拡張形式は[モデル／Rendererプラグイン](plugins.md)にあります。概念から順に読みたい場合は[音声合成の仕組み](how-utautts-speaks.md)を参照してください。

## 1. 中心となる考え方

UtauTTSは、UTAU音源に収録された原音を`oto.ini`に基づいて配置、時間伸縮、接続し、必要に応じて学習済みの韻律を加える連結型TTSです。原音の声質を保ちながら、文章の発音順に並べて音声を生成します。

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
    ├─ Prosody model ────── 10msピッチ曲線・モーラの音量
    └─ Plan builder ─────── 時刻付きの原音unit列
          │
          ▼
        render.Config
    ├─ UnitRenderer ─────── WORLD / Classic / external Provider
    └─ NeuralSynthesizer ── DiffSinger score + Provider session
          │
          ▼
        PCM + RenderReport
```

合成は次の順に進みます。

1. `frontend`が言語とphonemizerに応じて文章または読みを音素・モーラへ変換します。日本語の抑揚モデルを使う場合は、Open JTalk frontendからアクセント句や品詞なども受け取ります。
2. `voicebank`が`oto.ini`、`prefix.map`、subbankから原音候補を作り、隣り合う原音のつながりも考えてフレーズ全体の経路を選びます。
3. `prosody`がモーラ長、音量、ピッチを決めます。GUIで手動編集した値は自動予測より優先されます。
4. `render`が合成計画に従って原音を配置し、Rendererごとの方法でWAVへ変換します。

GUI、CLI、HTTP Serverは別々の音声処理を持たず、最終的には同じ`synth.Service`と`tts.Synthesize`へ到達します。入口を追加・変更するときは設定の伝わり方だけを確認し、音声処理を重複して実装しないようにします。

### 処理の段階

UTAU音源の経路は`tts.SynthesizeWithOptions`が次の順に実行します。DiffSingerなどのニューラル合成は別の経路です。

| 段階 | 担当 | 出力 |
| --- | --- | --- |
| 発音解析 | `frontend`、`tts`の言語profile | 読み、音素、強勢・声調など |
| 発話設計 | `prosody`、言語profile | 長さ・ピッチ・音量の予測 |
| 原音選択 | `voicebank`、`connection` | aliasと原音の選択 |
| 発話計画 | `tts/stage_plan.go`、`plan` | unitと音素の出力時刻 |
| ピッチ確定 | `tts/stage_pitch.go` | 描画用のピッチ曲線 |
| 描画 | `render`、選択したprovider | 音声と描画結果の診断 |

言語固有の読み、韻律、音素の長さは`languageProfile`にまとめ、WORLD固有の伸縮や接続は`render/worldline`で扱います。共通の`Mora`型は日本語のモーラ以外にも使うため、英語・中国語では名前だけで判断せず`Language`と`Phones`を参照します。

ピッチは指定済みの曲線を優先し、指定がなければ言語規則またはフレーム抑揚モデルから作ります。その後に言語別の境界音調、手動ピッチの順で加え、手動ピッチの後で曲線の変化量を制限します。発話計画は言語profileの音素長と予測から`plan.Build`で作り、作った後でユーザーのunit指定を適用します。

### 日本語の補正の流れ

原音の声質（包絡）を変える補正はありません。

| 段階 | 補正 | 場所 | 設定 |
| --- | --- | --- | --- |
| 原音選択 | 録音の無いヴ行・デュ・テュを同音の行で代替 | `voicebank`（`equivalentKanaForms`） | なし |
| 発話設計 | フレーム抑揚モデル、強さの拡大（2を超えると大きい動きほど広げる） | `prosody`、`tts`（`scaleAutomaticPitchCurve`） | `intonation_strength` |
| 発話設計 | 文末の音調 | `tts/japanese_boundary.go` | `boundary_tone` |
| 発話設計 | 文脈に応じたモーラ長（既定は無効） | `tts/japanese_duration.go` | `context_duration` |
| 発話計画 | 句読点の休止長 | `plan/pause_context.go` | `pause_context` |
| 発話計画 | CVVCで続く子音が長いときにVCを延ばす | `plan/plan.go`（`cvvcTransitionDuration`） | なし |
| 発話計画 | 原音の校正（固定部・有声開始・破裂の過渡） | `voicebank/speech_profile.go` | なし |
| 描画（本体） | 原音より大きく伸ばすときの伸びの抑制 | `render/base/timing.go`（`AdaptStretchTiming`） | `stretch_adapt` |
| 描画（本体） | 単独音の母音だけのモーラを前の母音から滑らかにつなぐ | `render/worldline/singlecv_legato.go` | なし |
| 描画（本体） | 子音の前後の小さな音高の動き | `render/worldline/microprosody.go` | `microprosody` |
| 描画（本体） | 時間伸縮の入力（音素区間と固定点のタイムライン） | `render/worldline/worldline.go`（`timingWarpJob`） | `timing_warp` |
| 描画（bridge） | 母音接続・破裂音の保護・同じ母音の隙間の補修 | `worldrender/world_speech.go`、`world_gap.go`、`stop_burst.go` | なし |
| 描画（bridge） | タイムラインに沿ってモーラの中だけ時間伸縮 | `speechtiming`、`worldrender/timing_warp.go` | `timing_warp` |

本体側の描画は`render/worldline/worldline.go`の`renderWorldlineEngine`が、タイミング、音高、原音の準備、素片、bridgeの実行の順に行います。bridgeへ渡す内容は`provider.UnitRendererJob`に明示し、bridgeは合成計画を読み直しません。日本語の連続音（単独音以外）は加工の少ない混合（同じ母音の隙間の補修つき）を、それ以外は音源に合わせて調整する混合を使います。

### 英語・中国語の原音写像

WORLDの英語・中国語では、まずotoと音響的な推定から原音の時間写像を作ります。試聴用の手動区間があればそれを優先し、なければ[原音区間ライブラリ](source-understanding.md)と照らし合わせます。手動区間が合わない場合はエラーにし、ライブラリに未登録の原音や合わない区間はotoの写像へ戻します。どちらも共通の`SourceSpan`型と`placeSourceSpan`で検証・描画し、対象の破裂音には過渡区間の保護を掛けます。適用した写像は描画結果の`speech_mapping`と、原音・出力のアンカーに記録します。英語の語末破裂音の閉鎖・解放の分離と、日本語の破裂音保護は常に有効です。

### 同梱Renderer

配布プロファイルによって利用できるものは異なります。

| ID | 概要 |
| --- | --- |
| `utautts-world-phrase` | 既定。原音ごとのWORLD特徴を共通の時間軸へ配置し、フレーズ全体を合成 |
| `classic-utau` | 選択したUTAU互換resamplerを実行し、wavtoolまたは内蔵処理で接続 |
| `diffsinger` | DiffSinger音源とbridgeを使うRenderer（Windows x64のFull配布のみ） |

## 3. テキストからモーラまで

### 読みの生成

読みが明示されていない場合は、ユーザー辞書を長い表記から順に適用してから、KagomeとIPA辞書で読みへ変換します。数字やラテン文字など内蔵の経路で発音を得られないトークンがあれば、Open JTalk frontendで読みを作ります。

日本語は`ja-kana`、英語は`en-arpasing`／`en-delta`／`en-vccv`／`en-cv`、中国語は`zh-cvvc`のphonemizerを使います。英語と中国語では日本語用の抑揚モデルを使わず、英語は強勢と句の規則、中国語はPinyinの声調曲線を使います。入力形式と制約は[日本語・英語・中国語の読み上げ](multilingual.md)にあります。

かな入力は`frontend.ParseKana`でモーラ列へ分けます。各モーラは少なくとも表記、子音、母音、休止かどうかを持ちます。促音、撥音、長音、拗音は文字単位ではなく合成単位として扱い、これより後の処理はモーラ列を参照します。

### Open JTalk特徴

日本語の抑揚モデルは、読みだけでは得られない次の特徴を使います。

- アクセント句内の位置と残り長
- アクセント核との位置関係
- 単語の開始と終了
- 品詞と品詞細分類
- 句境界と発話内位置

Go本体は同梱の`utautts-openjtalk-features`を別プロセスとして起動し、JSONで解析結果を受け取ります。helperはアプリの稼働中に常駐し、約100MBの辞書を最初の一度だけ読み込みます。PythonやOpen JTalkをGoのプロセスへ直接埋め込まないので、GUI、CLI、Serverから同じ実行ファイルを使えます。

helperは`tools/openjtalk-feature-bridge.py`をPyInstallerでまとめたもので、pyopenjtalkでNJDノードを作ります。NJDノードより後の処理（読み、モーラ、アクセント句の補正、疎な特徴）はGoの`internal/openjtalk`にも同じ実装があり、wasm版はブラウザのOpen JTalkからNJDを受け取ってこちらを使います。両者の一致は`go test ./internal/openjtalk -run TestCapturedPyopenjtalkBridgeParity`で確かめます。辞書からNJDを作る部分がGoに無いため、ネイティブ版はhelperを使い続けます。

Kagomeの読みとOpen JTalkのモーラ分割が一致しない場合は、完全一致、母音一致、長音、読み飛ばしに異なるコストを与える動的計画法で対応位置を求めます。対応しなかった位置には最も近い特徴を補います。単純に配列の添字で対応させると、未知語や長音より後のアクセント特徴がすべてずれます。

## 4. ボイスバンクの読み込みと原音選択

### メタデータ

ボイスバンクの読み込みでは、複数の`oto.ini`、UTF-8／Shift_JIS、`prefix.map`、`character.yaml`の基本的なsubbankを扱います。toneとcolorからsubbankを決め、そのprefix／suffixをaliasへ付けます。

空のprefixやsuffixも有効な設定です。文字列の前後の空白を一律に削ると、空の接辞を持つ有効な`prefix.map`の行を失い、別の音階の原音を誤って代わりに使うことがあります。メタデータの処理では「空として明示された値」と「設定が見つからない状態」を区別します。

### 候補ラティス

モーラごとに次の候補を作ります。

- VCV
- CV
- VC + CVの複合候補
- wildcardや表記違いの代替候補
- 促音の原音がない場合の無音`<closure>`

CVVCのVCは一つのモーラではなく、次のCVへ入る`transition` unitです。Plan上では主unitと分けて持ち、モーラ全体の時間は主unitで管理します。

`AliasPolicy=auto`は音源内のVC／VCVの収録比を見て、標準プロファイルかCVVC向けプロファイルを選びます。CVVC向けプロファイルではCVVC候補を優先し、transitionをsequential timingで置き、VCの音量を35%にします。英語では語境界の遷移を強めるため55%に上げます。明示指定したpolicyは自動判定より優先します。

候補のWAVは選択前に構造を検証します。存在しないWAV、読めない形式、成り立たない切り出し範囲などは候補から外し、理由をPlanへ残します。候補数は各位置で最大32件に制限し、組み合わせが増えすぎるのを防ぎます。

### 経路選択

標準では、発話の休止区間ごとにViterbi探索を行います。概念上の経路スコアは次の和です。

```text
path score = Σ local candidate score + Σ adjacent join score
```

各候補のスコアには、代替候補へ落ちた段階、`oto.ini`の値の整合性、subbankや形式の優先度が入ります。`oto.ini`の値の評価は言語にも依存します。英語のC+VやVCCVのように子音と母音を分けて録音する音源では、母音のoverlapがpreutteranceを超える設定が仕様なので、慣習違反として減点しません。

接続のスコアでは、隣り合う原音の音量、スペクトル、F0などの境界特徴と、同じ録音グループかどうかを評価します。同じWAVの中で前へ進む原音には、距離が近いほど大きい6〜9点を加点します。複数のモーラを一つのファイルへ収録したVCVやVCの音源でも遷移音を選び損ねないよう、下限の6点を確保しています。後ろの原音の始まりが閉鎖区間になるVCVでは減点を弱め、母音側の候補のスコアを優先します。経路はこれらの手設計のスコアを使ったViterbi探索で決めます。

候補が少ないUTAU音源では、境界の連続性だけを優先すると音素文脈や声質が変わることがあります。そのため候補はphonemizerと音源側の指定から音素文脈に合うものだけを作り、接続のスコアはその候補の中での順位付けにだけ使います。同じ音素文脈の表記違い（例: 英語C+Vの文中で試す`- V`と`V`）では、前後の音響によって順位が入れ替わることがあります。

## 5. Plan（段階間の受け渡し）

`internal/plan`のPlanは、原音選択とRendererを分ける中間表現です。各unitには次の情報が入ります。

- 元のモーラ位置とunitの役割
- alias、WAV、`oto.ini`のファイルと行
- 発音開始時刻とモーラ長
- offset、consonant、cutoff、preutterance、overlap
- pitchとenergyの係数
- 候補・接続・累積の経路スコア
- 代替段階、subbank、tone、color
- 候補の除外理由と音響スコア
- Rendererへ渡す前の選択時の値。Rendererが計算した実効値は`RenderReport`へ分ける

Planは、候補選択、時間設計、Rendererの差を切り分けるための再現可能な記録です。新しい自動補正では、入力値、実際に使った値、採用した理由、代替へ戻した理由を記録します。最終的なWAVとPlanを合わせて見ると、品質が下がった原因を追えます。

共有のPlanは読み取り専用で扱います。`UnitRenderer`にはPlanのコピーを渡し、Rendererが計算した先頭の余白、境界の補修、実効のpreutterance／consonant／overlap、原音と目標のF0などを`RenderReport`として返します。CLIのPlan JSONやLABのように描画後の値が必要な出力だけが、`tts.Result.RenderedPlan()`でコピーへReportを適用します。

## 6. 韻律モデル

### モデル形式

モデルは任意のコードではなく、重みとメタデータを持つ自己記述JSONです。`id`、`version`、`feature_version`、`mode`からGo側の決まった推論器を選びます。未知の形式、壊れたshape、IDの重複は読み飛ばさず、カタログを作る時点でエラーにします。

同梱モデルは次のとおりです。

| モデル | 形式 | 出力 |
| --- | --- | --- |
| `frame-intonation-tcn-v12` | `base_model`（v10）＋`f0_head` | 10ms単位の相対ピッチとモーラの音量（既定） |
| `frame-intonation-tcn-v11` | `base_model`（v10）＋`f0_head` | 10ms単位の相対ピッチとモーラの音量 |
| `frame-intonation-tcn-v10` | version 8 / feature 1 | 10ms単位の相対ピッチ |
| `frame-intonation-tcn-v9.1-t` | version 8 / feature 1 | 10ms単位の相対ピッチ |
| `frame-intonation-tcn-en-v1` | version 8 / feature 1 | 英語の10ms単位の相対ピッチ |
| `tone-intonation-zh-v1` | version 13 / feature 1 | 中国語の声調曲線への範囲を限ったピッチ補正 |

frame headは、モーラとOpen JTalk由来の特徴をフレームへ展開し、dilationを持つ小型のTCNで相対ピッチを予測します。`frame-intonation-tcn-v9.1-t`は440〜457特徴、10ms間隔、学習時の出力範囲±250 centです。推論後はモデル内の強さ、平滑化、percentile／最大値の制限を適用し、学習音声に由来する細かなF0の揺れを抑えます。

抑揚の強さ（既定4、0〜8）は、2までは曲線に一律の倍率を掛けます。2を超える分は、強さ2の曲線の値`x`に`1 + (強さ/2 - 1)(1 - exp(-(x/100)^2))`を掛けます。0付近の平らな部分は強さ2のままで、アクセントや句の上がり下がりのような大きな動きほど指定の強さへ近づきます。自動の抑揚曲線が無いときの音源ピッチの安定化には、強さを4までで使います。

multitaskモデル（version 10 / feature 2 / mode `prosody_multitask_tcn`）は、frame headに加えてモーラ長の倍率を出す`mora_duration` headを持ちます。絶対的なmsではなく基準モーラ長に対する倍率なので、GUIの話速設定や音源の違いと両立します。標準配布にはversion 10のモデルを含みません。

英語モデルはARPAbetから得た強勢、語境界、句境界を特徴にします。中国語モデルはPinyinの声調に基づく曲線を補正します。カードの言語を変えた場合は、対応するモデルがあれば切り替えます。

manual residual形式（version 11）も実行時に解釈できます。v8を基準に、GUIで行った手修正の傾向だけを小さなcentの補正として学習する形式です。元モデルのSHA-256と補正範囲を持ち、基準モデルへ残差を加えます。標準配布にはこの形式のモデルを含みません。

### 推論順序

1. モーラ列とOpen JTalk特徴を作ります。
2. モーラ単位の長さ、ピッチ、音量の予測を作ります。
3. 手動のモーラ長がある位置は、自動の長さより優先します。
4. 確定した長さからPlanと各モーラの時刻を作ります。
5. その時刻に合わせて10msのフレームピッチを作ります。
6. 自動ピッチへ手動ピッチを加算するか、置き換えます。
7. Rendererの機能と`ApplyPitch`を確認して波形へ適用します。

GUIの解析プレビューも同じ順序を使います。自動値を0などの特別な値で隠さず、予測した値を画面へ返します。ユーザーが編集した位置だけを上書き値として持ち、再生時に解析し直しても表示値が変わらないようにしています。

### 相対ピッチ

モデルが出力するのは話者の絶対的なF0ではなく、発話内の基準に対するcent値です。

```text
cents = 1200 × log2(F0 / reference F0)
```

Rendererは各原音のF0を測り、この相対曲線を音源側の声域へ重ねます。学習した話者の声の高さはボイスバンクへ移さず、抑揚の形だけを使います。

## 7. Renderer

Renderer manifestの`id`は、保存データやUIで使う公開識別子です。カタログはこの公開IDを`engine.ResolvedEngine`へ解決し、`contract`、`provider`、provider version、型付きのresource、platform、capabilityを検証します。manifestは表示情報と実行時のresourceを宣言し、ネイティブコードや新しいエンジンのABIはGo側に実装します。標準Rendererも`renderer/<id>/renderer.json`から読み込み、実行時には`manifest_version: 2`だけを読み込みます。`utau-external-resampler` providerは`Resamplers/`と`Wavtools/`の実行ファイルを組み合わせます。

Renderer IDを省略した場合だけ、カタログの既定Rendererを使います。未知のIDや、必要なファイルが不足しているRendererを明示した場合はエラーになります。

設定もRenderer単位で分けます。`tts.Config`は、テキスト、音源、モデル、Planの作成に必要な共通の入力と、解決済みの`engine.ResolvedEngine`を持ちます。Classicの実行ファイルやWORLD専用の設定は`render.Config`の`render.ProviderOptions`へ分け、Classicは`ClassicOptions`、WORLDは`WorldlineProviderOptions`にまとめます。設定の既定値・範囲・対象のproviderは設定表（`internal/settings`）が持ち、`synth`の適用テーブルが値をそれぞれへ振り分けます。

### Classic UTAU Renderer

`classic-utau`は内部で`utau-external-resampler`を使います。OpenUTAU由来の音素タイミングに従ってresamplerをunitごとに呼び、tone、offset、必要長、consonant、cutoff、12bit形式のピッチ列などを渡します。返されたWAVはサンプリング周波数をそろえ、内蔵処理または選択したwavtoolで接続します。

resamplerとwavtoolは独立したプロセスです。呼び出しごとに、終了コード、出力WAV、タイムアウトを確認します。velocity、flags、volume、modulation、tempoはunit単位でも指定できます。

### UtauTTS WORLD phrase

`utautts-world-phrase`は現在の既定Rendererです。OpenUtauの`PhraseSynth`は使わず、公式WORLDのHarvest、CheapTrick、D4C、Synthesisだけを信号処理の部品として使います。原音の切り出し、子音を保つ時間写像、前後のフェード、特徴量の補間と重なりの処理はUtauTTS側にあります。

原音ごとのF0、スペクトル包絡、非周期性指標はbridgeの中にキャッシュします。再生時はこれらをフレーズの10 msの時間軸へ置き直し、隣り合う分析フレームを補間してから、一度だけWORLDで合成します。

日本語のF0曲線には、子音の種類ごとの小さな音高の動き（`microprosody.go`）を足します。WORLDは元の録音のF0を目標の曲線で置き換えるため、無声破裂音の直後に母音の頭が少し高くなる（約+60セント、20〜30 ms）といった動きが失われます。つくよみちゃんコーパスとみんなで作るJSUTから、150 msの移動中央値からの残差を子音の種類ごとに平均し、母音が続く場合の平均を引いたテンプレート（母音の開始の−20〜+40 ms）を作って掛けます。この補正（`microprosody`）、境界音調（`boundary_tone`）、伸縮の抑制（`stretch_adapt`）は既定で有効です。

日本語・英語・中国語では、合成の直前に時間伸縮（`internal/speechtiming`）を行います。言語別の目標モデル（`speech-timing-target-v1` / `-en-v1` / `-zh-v1`）が、合成計画の音素・長さと相対F0から、話者ごとに正規化した対数メル包絡の軌跡（読み上げで音が移り変わる速さ）を予測します。

- 音素区間は、日本語では子音を「ノートの開始−実効の先行発声」から始め、前のモーラの母音をそこで切ります。CVVCでは子音がVCから始まるので、子音の長さにVCの長さを含めます。英語・中国語はプランの`phone_timings`（codaを含む音素区間）をそのまま使います。
- 音素区間と伸縮の固定点は本体で計算し、bridgeへ渡します。
- 出力の包絡も同じ80帯域へ変換して正規化し、モーラの開始と発声区間の終わりを固定点にして、区間ごとのDTWで対応を求めます。伸縮は0.5〜2倍に制限して平滑化します。
- 特徴量は対応に沿って並べ直すだけで、包絡の形は変えません。句の最後のモーラは子音を含めて伸縮せず、その手前60 msでなだらかに元へ戻します。

推論は純GoのTCNで、`Predictor`インターフェースの後ろにあるため、gogradなど別の推論系へ差し替えられます。重みはsafetensors（F32）でbridgeに埋め込みます。学習はgogradを使うGoのコマンド`cmd/tools/train-speech-timing`で行います（[手順](model-training.md)）。

日本語の抑揚の既定は`frame-intonation-tcn-v12`です。v12はv10の曲線を混ぜず（`base_blend` 0）、Irodori-TTSの読み上げで学習した自然スケールのF0ヘッドの曲線だけを使い、曲線全体を50セント下げます（`pitch_offset_cents` -50）。教師のF0は、オクターブの誤推定を折り返して作り直しています。v11は、`frame-intonation-tcn-v10`の抑揚曲線（強さ4で拡大、重み0.65）と、Irodori-TTSの読み上げで学習した自然スケールのF0ヘッドの曲線（重み0.35、既定の強さ4で等倍）を混ぜます（モデルJSONの`base_blend`）。同じモデルのエネルギーヘッドで、モーラの音量（0.75〜1.3倍）も変えます（`use_energy`）。音量を変えるv11は、音量を変えないv11との聴取比較で10対5で選ばれました。

F0ヘッドは時間伸縮と同じ系統のモデル（`internal/speechtiming`）です。モデルJSONは基準の抑揚モデル（`base_model`）とF0ヘッドの重み（`f0_head`）を持ち、アクセント特徴とモーラの予測は基準モデルが行います。F0ヘッドの入力は音素の前後関係・音素内の位置・長さ・アクセント・品詞で、前後約5秒を見ます。自然スケールのモデルは抑揚の強さを「強さ/既定の4」で掛けます。既存の抑揚モデルの輪郭を教師にした蒸留モデル（`f0_scale`あり）は、教師の後処理（発話区間のGaussian平滑化20 ms、p99 75セント、最大90セントのクリップ）を推論時にも適用します。エネルギーヘッドはモーラごとの音量係数（0.75〜1.3）をプランの`EnergyFactor`へ適用します。GUIの高さの表示、DiffSingerへの入力、USTXの書き出しも、合成と同じ休止長で入力を組み、合成と同じF0ヘッドの曲線を使います。日本語以外では言語別の抑揚モデルをそのまま使います。学習は`train-speech-timing`の`--f0`と`--plan-augment`で行います（[手順](model-training.md)）。

### DiffSinger

`diffsinger`は専用のDiffSinger音源を`dsconfig.yaml`から読み込み、Windows x64のbridgeを通じて音響モデルとvocoderを実行します。通常のUTAU音源の`oto.ini`、resampler、wavtoolを使う経路とは異なります。対応範囲と配置は[DiffSinger](diffsinger.md)を参照してください。

### 変更する場所の分担

Rendererは選択済みのunitのaliasを変えません。候補選択の改善は`voicebank`、時間付きのunit列の変更は`plan`、波形処理の変更は`render`で行います。この分担により、同じPlanを複数のRendererへ渡して比べられます。

内蔵のProviderと外部のProviderは、同じ`UnitRenderer`インターフェースで扱います。内蔵側もPlanのコピーへ処理を行い、外部側は`utautts-provider`のハンドシェイクと共通のjobを通じて音声と診断を返します。DiffSingerはUTAUのunit列を入力にするRendererではなく、`neural-synthesizer` contractの`NeuralScore`経路を使います。

## 8. GUI、CLI、Serverとキャッシュ

Qt GUIはQMLからC ABIで`internal/native.Engine`を呼び出します。HTTPやWebViewは介さず、音源の列挙、解析、韻律のプレビュー、合成、exo出力をメソッド名とJSONでやり取りします。

CLIとHTTP Serverも同じRendererカタログと`synth.Service`を使います。モデルとRendererはファイル名ではなくIDで選び、明示したディレクトリ、配布物内のディレクトリ、開発用のワークスペースの順に探します。必要なファイルの相対パスは、Rendererディレクトリを基準に絶対パスへ変換します。

編集のたびに合成し直す負担を減らすため、次をプロセス内にキャッシュします。

- 読み込み済みのボイスバンク
- サイズと更新時刻で確かめたモデル
- 最大512件のOpen JTalk解析
- 最大256MiBのデコード済みWAV（LRU）

音源を読み込み直すときは、これらを明示的に破棄します。新しいキャッシュを追加するときは、ファイル更新の検知、上限、明示的な消去、並行アクセスの4点をそろえてください。

外部helperとProviderの寿命は、キャッシュとは別に管理します。Open JTalk helper、WORLD bridge、DiffSinger bridge、セッションに対応した外部Providerは初回の利用時に起動し、同じEngineで複数の合成要求に使い回します。合成ごとにプロセスを起動し直すことはせず、キャンセル／タイムアウトやプロセスの終了時だけ再起動します。Classic UTAUのresamplerとwavtoolは、現状ではunitごとにプロセスを起動します。

合成要求のキャンセルには`context.Context`を使います。外部helperにはタイムアウトと終了待ちの時間があり、長いunitの処理中も定期的にキャンセルを確認します。GUIだけでなく、Serverの切断や終了処理でも同じ方法を使います。

## 9. 設計上の制約

### `oto.ini`の扱い

`oto.ini`は、原音の切り出し、子音の固定部、先行発声、overlapを記述する基礎の情報です。Rendererはこれを使わずに波形だけから境界を推定することはせず、補正も元の定義から大きく外れない範囲に制限します。

原音をTTS向けの短い長さへ強く圧縮すると、震え、途切れ、子音の欠けが起きます。固定のmsだけでなく、原音の長さに対する圧縮率と子音部の保護も考える必要があります。

### 連続した時間写像の適用範囲

同じ録音WAVの中で隣り合う原音は、候補選択で連続性を考慮します。原音を分割せず、連続した時間写像へ置き換える方式は品質が高いものの、対応する録音の並びと境界の管理が必要なため、標準のRendererでは使う条件を限っています。

### 波形補修と候補選択の限界

波形補修は波形に残っている情報だけを入力にでき、候補選択は収録済みの音素文脈だけを対象にできます。補修やスコアの追加条件は、候補の多さ、原音の文脈、適用する位置に依存します。

## 10. 変更時の方針

### 公開IDと既存経路の維持

既存RendererのID、意味、出力は維持します。新しい方式は、別のRenderer IDか、初期状態で無効の明示的なオプションとして追加します。評価が不十分な段階でも、同じ入力で標準の出力を再現できる状態を保ちます。

### 代替処理の扱い

新しい制御値には範囲の制限を設けます。入力検証では、NaN、単調に増えない時刻のアンカー、大きすぎる切り出し、必要なファイルの不足をエラーにします。Renderer IDを省略した場合はカタログの既定Rendererを使い、未知の明示IDはエラーにします。信頼度の低いunitや境界だけを代替処理へ戻す場合は、その位置と理由をPlanへ残します。
