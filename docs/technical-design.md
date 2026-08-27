# UtauTTS 技術設計ガイド

この文書はUtauTTSの内部構造と現在の設計に至った研究上の判断を、将来の開発者向けにまとめたものです。コマンドの使い方は[CLI](cli.md)、外部から見える構成は[構成](architecture.md)、拡張形式は[モデル／Rendererプラグイン](plugins.md)にあります。

## 1. 中心となる考え方

UtauTTSはテキストから音声波形を生成するニューラルTTSではありません。UTAUボイスバンクに収録された原音を選び`oto.ini`を基準に配置・時間伸縮・接続して必要な場合だけ学習済みの韻律を加える連結型TTSです。

この方式では品質を次の順で守ります。

1. 文章と音素を正しく聞き取れること
2. 子音が欠けず、接続クリックや途切れが目立たないこと
3. ボイスバンク固有の声質を壊さないこと
4. リズムとイントネーションが自然であること

抑揚が不自然でも文章は聞き取れますが子音が脱落すると発話内容そのものが変わります。なので滑らかさだけを改善して明瞭度を失う変更は採用しません。

## 2. システム全体

```text
GUI / CLI / HTTP Server
          │
          ▼
      synth.Service
          │ 設定、音源、モデル、RendererをIDから解決
          ▼
    ┌─ Japanese frontend ── 読み・モーラ
    ├─ Open JTalk helper ── アクセント・単語・品詞特徴
    ├─ Voicebank resolver ─ 候補ラティスと選択経路
    ├─ Prosody model ────── モーラ長・10msピッチ曲線
    └─ Plan builder ─────── 時刻付きの原音unit列
          │
          ▼
       Renderer
    ├─ waveform ─────────── Go内の波形接続
    ├─ Classic faithful ─── .NET bridge + Worldline Resample
    └─ WORLDLINE-R ──────── .NET bridge + Worldline PhraseSynth
          │
          ▼
        PCM WAV
```

主要な責務は次のように分かれています。

| 領域 | 主な場所 | 責務 |
| --- | --- | --- |
| 共有オーケストレーション | `internal/synth`、`internal/tts` | 入力検証、各段階の呼び出し、GUI・CLI・Server間の挙動統一 |
| 日本語解析 | `internal/frontend`、`internal/openjtalk` | 読み、モーラ、アクセント句、単語境界、品詞 |
| ボイスバンク | `internal/oto`、`internal/voicebank` | `oto.ini`、`prefix.map`、subbank、alias候補、原音選択 |
| 韻律 | `internal/prosody`、`models` | モーラ長、相対ピッチ曲線、手動補正 |
| 合成計画 | `internal/plan` | Rendererに依存しない時刻付きunit列と診断情報 |
| 波形生成 | `internal/render` | 時間伸縮、ピッチ処理、包絡、mix、Worldline bridge呼び出し |
| 拡張カタログ | `internal/plugin`、`plugins/renderers` | Rendererとモデルの発見、ID、能力、asset解決 |
| GUI境界 | `internal/native`、`cmd/utautts-native`、`qt` | QMLからC ABI経由で共有合成処理を呼ぶ |
| HTTP API | `internal/api`、`cmd/utautts-server` | 共有合成処理をHTTPとして公開する |

GUI、CLI、Serverは別々の合成実装を持ちません。最終的には同じ`tts.Synthesize`へ到達するので機能追加では入口ごとの設定伝播を確認し、音声処理を重複実装しないようにします。

## 3. テキストからモーラまで

### 読みの生成

読みが明示されていない場合は最初にユーザー辞書を長い表記から順に適用してKagomeとIPA辞書で読みへ変換します。数字やラテン文字など内蔵経路で発音を得られないトークンがあればOpen JTalk frontendへフォールバックします。

かな入力は`frontend.ParseKana`でモーラ列へ分解されます。各モーラが持つのは少なくとも表記、子音、母音、休止かどうかです。促音、撥音、長音、拗音を文字単位ではなく合成単位として扱うのでこの段階より後ろでは元のUnicode文字列を直接走査しません。

### Open JTalk特徴

`frame-intonation-v8`などのモデルは読みだけでは得られない次の特徴を使います。

- アクセント句内の位置と残り長
- アクセント核との位置関係
- 単語の開始と終了
- 品詞と品詞細分類
- 句境界と発話内位置

Go本体は同梱された`utautts-openjtalk-features`を別プロセスとして起動してJSONで解析結果を受け取ります。PythonやOpen JTalkをGoプロセスへ直接埋め込まないのでGUI、CLI、Serverから同じ実行形式を利用できます。

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

CVVCのVCは一つのモーラではなく次のCVへ入る`transition` unitです。Plan上では主unitと分けて保持しますがモーラ全体の時間を余分には増やしません。

`AliasPolicy=auto`は音源内のVC／VCV収録比を見て従来互換かCVVC向けプロファイルを選びます。CVVC向けプロファイルではCVVC候補を優先してtransitionをsequential timingで置き、VC音量を35%にします。明示指定されたpolicyは自動判定より優先です。

候補WAVは選択前に構造検証されます。存在しないWAV、読めない形式、成立しない切り出し範囲などは候補から外して理由をPlanへ残します。候補数は各位置で最大32件に制限して組合せ爆発を防ぎます。

### 経路選択

標準は発話の休止区間ごとに行うViterbi探索です。概念上の経路スコアは次の和です。

```text
path score = Σ local candidate score + Σ adjacent join score
```

local scoreにはaliasのfallback段階、`oto.ini`値の整合性、subbankや形式の優先度が入ります。任意の音響選択モードでは候補のRMS、F0、有声率などから得た保守的な補正も加わります。

join scoreは隣接原音のenergy、スペクトル、F0などの境界特徴と同じ録音groupかどうかを評価します。通常は手設計scoreを使い研究用途なら学習済みjoin-cost JSONへ差し替えられます。`greedy`と`target-only`は診断・互換比較用です。

候補が疎なUTAU音源では境界の連続性を優先すると音素文脈や声質が変わることがあります。なのでjoin scoreは候補の言語的な適合性を置き換えず保守的な補助値として扱います。

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
- Rendererが実際に使ったtimingとF0

Planは単なるデバッグ出力ではありません。候補選択、時間設計、Rendererのどこで差が生じたかを切り分けられる再現可能な記録です。新しい自動補正を加える場合は入力値、実効値、採用理由、fallback理由をPlanへ記録してください。最終WAVだけでは退行原因を追えません。

## 6. 韻律モデル

### モデル形式

モデルは任意コードではなく重みとメタデータを持つ自己記述JSONです。`id`、`version`、`feature_version`、`mode`からGo側の決定論的な推論器を選びます。未知形式、壊れたshape、ID重複は読み飛ばさずカタログ構築時のエラーにします。

現在同梱するモデルは次の2つです。

| モデル | 形式 | 出力 |
| --- | --- | --- |
| `frame-intonation-v8` | version 8 / feature 1 | 10ms単位の相対ピッチ |
| `prosody-multitask-v1` | version 10 / feature 2 | v8系ピッチとモーラ長倍率 |

v8のframe headはモーラとOpen JTalk由来特徴をフレームへ展開してdilationを持つ小型TCNで相対pitchを予測します。同梱モデルは406特徴、10ms間隔、学習出力範囲±250 centです。推論後にはモデル内のrender strength、平滑化、percentile／最大値制約を適用して学習音声の細かなF0揺れをそのまま強制しないようにします。

multitaskモデルは同じframe headへ423特徴からモーラ長倍率を出すduration headを加えたものです。絶対msではなく基準モーラ長に対する倍率なのでGUIの話速設定や音源差と共存できます。

version 11のmanual residual形式もruntimeが解釈できます。これはv8を基準にGUIで行った人手修正の傾向だけを小さなcent補正として学習する形式です。元モデルのSHA-256と補正範囲を持ち、基準モデルを置き換えず残差を加えます。標準配布モデルにはまだ含まれていません。

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

モデルは話者の絶対F0を生成せず発話内基準に対するcent値を出します。

```text
cents = 1200 × log2(F0 / reference F0)
```

Rendererは各原音のF0を測ってこの相対曲線を音源側の声域へ重ねます。学習話者の声の高さをボイスバンクへコピーせず抑揚の形だけ借りるわけです。

## 7. Renderer

Renderer pluginの`id`は保存データやUIで使う公開識別子、`backend`はGoに組み込まれた実装識別子です。現在のpluginは設定とassetを宣言するもので任意のnative codeを動的ロードする仕組みではありません。

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

### OpenUTAU Classic faithful

`openutau-classic-worldline-faithful`はOpenUtau Classicに近いphone timing、Worldlineによるunit再合成、5点envelopeを組み合わせるRendererです。現在の既定は後述する`openutau-worldline-r-faithful`でClassic faithfulは比較や音源との相性に応じて選べます。

Go側は次を行います。

- 前後unitの長さからpreutteranceとoverlapを安全範囲へ調整
- 発話先頭を負時刻へ配置できるようphrase先頭marginを計算
- 原音F0の測定とoctave安定化
- 相対pitch曲線を原音ごとの局所F0へ重ねる
- required length、skip、tone、consonant、5点envelopeをmanifestへ記録

manifestは自己完結.NETアプリのWorldline bridgeへ渡されます。bridgeは`worldline.dll`を読み込んで原音ごとに分析・再合成してから共通phrase時刻へmixし、PCM WAVをGoへ返します。このプロセス境界があるのでGo／Qt側はOpenUtau由来native ABIへ直接依存しません。

required lengthはClassic方式に合わせて50ms単位へ丸められて子音部は母音部と別の時間規則で扱われます。5点envelopeはpreutterance、前後のoverlap、次unitのtail intrusionを表して単純な一定長crossfadeより子音の流入と母音末尾を保ちます。

CUDA版は同じPlan、timing、Worldline条件を使って対応DLLで5点envelopeによるmixをGPU化します。意味上はCPU版と同じですが環境依存機能なのでexperimentalとして配布されます。

### OpenUTAU WORLDLINE-R faithful

`openutau-worldline-r-faithful`は現在の既定Rendererです。OpenUtau 0.1.565のnative `PhraseSynth` APIを使います。Classic faithfulと同じphone timingを入力にしますが原音ごとの完成波形を重ねるのではなく、各原音のWORLD特徴を共通時間軸へ配置してからフレーズ全体を一度だけ合成します。

Go側は先行発声を含むフレーズ時刻、絶対F0曲線、各unitの`position`、`skip`、`length`、fadeをmanifestへ記録します。.NET bridgeは原音とFRQを`PhraseSynthAddRequest`へ渡してgender、tension、breathiness、voicingの既定曲線とF0を設定し`PhraseSynthSynth`を呼びます。

WORLDLINE-Rの主要処理は`worldline.dll`内部で完結します。Classic CUDA版のGPU mixは利用できないのでWORLDLINE-R faithfulにCUDA版はありません。

### Rendererを変更するときの境界

Rendererは選ばれたunitを受け取って別のaliasへ勝手に変更しません。候補選択を改善する場合は`voicebank`、時間付きunit列を変える場合は`plan`、波形処理だけを変える場合は`render`へ実装します。この境界を守れば同じPlanを複数Rendererへ渡して比較できます。

## 8. GUI、CLI、Serverとキャッシュ

Qt GUIはQMLからC ABIで`internal/native.Engine`を呼びます。音源列挙、解析、韻律preview、合成、exo出力はmethod名とJSONでやり取りします。HTTPやWebViewはGUI内部の合成経路には使いません。

CLIとHTTP Serverも同じplugin catalogと`synth.Service`を使います。モデルとRendererはファイル名ではなくIDで選択して明示directory、配布物内directory、開発workspaceの順に解決します。assetの相対pathはplugin directoryを基準に絶対pathへ変換します。

反復編集を軽くするため次をプロセス内でcacheします。

- 読込済みボイスバンク
- サイズと更新時刻で検証したモデル
- 最大512件のOpen JTalk解析
- 最大256MiBのデコード済みWAV LRU

音源再読込ではこれらを明示的に破棄します。新しいcacheを追加するときはファイル更新の検知、上限、明示clear、並行アクセスの4点を揃えてください。

合成要求は`context.Context`でキャンセルされます。外部helperにはtimeoutと終了待ち時間があり長いunit処理中も定期的にキャンセルを確認します。GUIだけでなくServerの切断や終了処理でも同じ仕組みを使います。

## 9. 設計上の制約

### `oto.ini`は捨てるべき初期値ではない

`oto.ini`は原音の切り出し、子音固定部、先行発声、overlapを記述する基礎情報です。Rendererはこれを捨てて波形だけから境界を推定することはなく、必要な補正も元の定義から大きく外れない範囲に制限します。

原音をTTS向けの短い長さへ強く圧縮すると震え、途切れ、子音欠落が生じます。固定msだけでなく原音長に対する圧縮率と子音部の保護も考える必要があります。

### 連続anchorは高品質でも適用範囲が狭い

同じ録音WAV内の隣接原音は分割せず連続した時間写像を使える場合があります。ただし対応する録音列は限られて局所的な方式切替によって新しい境界も生まれるので標準Rendererには統合していません。

### 波形補修と候補選択には上限がある

波形補修は入力に残っている情報しか利用できず候補選択は収録されていない音素文脈を生成できません。補修やscoreを追加する前に候補密度、原音の文脈、適用できる位置を確認します。

## 10. 変更時に保つ互換性

### 公開IDと既存経路を維持する

既存の正式Rendererの意味と出力を新実験のために変更しないでください。新方式は別backend／plugin IDか既定offの明示オプションとして追加します。実験が失敗しても同じ入力で以前のWAVへ戻れる状態を保ちます。

### fallbackを局所化し、理由を残す

新しい制御値には範囲制限を設けます。NaN、非単調なtime anchor、過大なcrop、asset不足などを黙って通しません。実験Rendererを明示選択したのにassetがない場合も別Rendererへ黙って切り替えずエラーにします。低信頼度のunitや境界だけをfallbackする場合はその位置と理由をPlanへ残します。
