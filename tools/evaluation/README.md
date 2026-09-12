# 読み上げ品質の評価

`tts-eval`は固定した文章で読み・原音選択・合成音声を比較する開発者向けツールです。リポジトリのルートから実行します。[開発環境](../../docs/building.md)と評価対象のボイスバンクを用意してください。

以下の音源パスは手元のボイスバンクに置き換えてください。出力先には毎回新しいディレクトリを指定します。

## 読みと原音候補を診断する

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/english-bank" --corpus tools/evaluation/english-v1.json --diagnose --out out/english-diagnosis
go run ./cmd/tools/tts-eval --voicebank "./voice/chinese-bank" --corpus tools/evaluation/chinese-v1.json --diagnose --out out/chinese-diagnosis
```

英語と中国語のコーパスには自動読みと明示読みのケースがあります。数字や多音字も含みますが発音を網羅する評価セットではありません。各ケースの`language`と`phonemizer`を音源に合わせてください。英語の既定値は`en-arpasing`です。Delta/VCCV音源では`en-delta`または`en-vccv`を指定します。

`english-v2.json`は無強勢母音と単語をまたぐ子音群を含む8ケースです。`chinese-v2.json`は鼻音韻尾・üの表記差・変調を含む9ケースです。`--corpus`で指定します。v1は過去の比較用に残しています。中国語v2の鼻音ケースは`ban/bang`などの完全な音節を使います。v1の単独韻母が見つからない結果と区別してください。

`diagnostics.json`には読みと発音単位に加えて原音名（alias）の候補を保存します。原音WAVは読みますが音声合成は行いません。音素化・候補探索の失敗や必須音の不足があれば全ケースの処理後に終了コード1を返します。

| 項目 | 意味 |
| --- | --- |
| `coverage.positions` | 休止を除く位置数 |
| `coverage.covered` | 有効な主候補がある位置数 |
| `coverage.candidate_counts` | 候補絞り込み後の件数。添字は休止を含む発音単位に対応 |
| `coverage.missing` | 主候補の不足位置と探索したaliasや除外理由 |
| `coverage.missing_phones` | 最も不足が少ない候補でも欠ける必須音 |
| `lattice` | 経路選択結果。全文の探索に失敗した場合は保存しない |

促音の無音区間も有効候補に含みます。候補が揃っていても発音の正しさは保証されません。原音の収録内容と接続は聴取で確認してください。

## 音声を比較する

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/japanese-bank" --renderers waveform --model none --repeat 1 --out out/ja-base
go run ./cmd/tools/tts-eval --voicebank "./voice/japanese-bank" --renderers waveform --model none --repeat 1 --speech-timing --out out/ja-speech
```

コーパスの既定値は`tools/evaluation/japanese-v1.json`の8文です。`--model none`は学習済み抑揚モデルを使いません。任意のモデルは`--model-file`でJSONのパスを指定できます。発話タイミング設定の効果と制約は[発話タイミングと音源校正](../../docs/speech-quality-experiment.md)を参照してください。

英語・中国語でも`--corpus`を指定して比較できます。診断のみの場合と異なり`--diagnose`は付けません。WORLDで比較する場合は`--renderers utautts-world-phrase`を指定します。対応するランタイムをビルドして用意してください。ブリッジのパスは`--bridge`で指定できます。

出力にはWAV・TXT・LABと合成計画（Plan JSON）および`report.json`を含みます。合成計画の時刻は生成時の目標値です。音声から実測した境界ではありません。

| 診断項目 | 意味 |
| --- | --- |
| Planの`phone_timings` | 発音単位ごとの目標時刻 |
| Planの`missing_phones` | 選択した経路で不足した必須語尾や検出可能な句頭子音群 |
| unitの`speech_profile` | 固定部の元の値と提案値。安定区間・F0・音量・有声率・信頼度・採否の理由 |
| unitの`speech_retime_applied` | waveformまたはCPU版WORLDの区間別伸縮を適用したか |
| unitの`speech_join_applied` | CPU版WORLDの母音接続補修を適用したか |
| unitの`effective_consonant_ms` | 伸縮後の固定部の位置 |
| Planの`boundary_repair_decisions` | waveformの接続補修の候補数と採否。補修前後の波形差分指標 |
| reportの`missing_phone_groups` | 必須音が不足したグループの件数 |

任意のリリース音は必須音の不足と区別します。語中の複雑な子音群や原音の発音誤りをすべて検出できるわけではありません。音声生成では代替候補を使うため生成成功だけでは音の欠落を判断できません。

子音と接続の比較には`--corpus tools/evaluation/japanese-connection-v1.json`を指定します。同じ読みを使う6ケースで母音連続・破裂音・摩擦音・鼻音・長母音を確認できます。単独音と連続音など複数の収録形式で試してください。

出力するPlan JSONには合成後の診断情報を含みます。区間別伸縮を適用しても指定したモーラ長は変えません。`boundary_repair_decisions`の指標が改善しても自然さが向上したとは限らないため補修箇所を試聴してください。

CPU版WORLDは音響特徴の時間軸と母音接続を調整します。`--renderers utautts-world-phrase --speech-timing`で比較できます。本体とブリッジの両方をビルドしてください。解析キャッシュを使う2回目の合成も確認する場合は`--repeat 2`を指定します。

## 読み・長さ・ピッチを固定する

コーパスはケースの配列です。`reading`で読みを固定できます。`mora_durations_ms`はミリ秒単位の長さ配列です。配列の要素数は診断結果の発音単位数に合わせてください。単位はphonemizerによって異なります。

`pitch_curve`は一定間隔のピッチ列です。`frame_ms`にフレーム間隔をミリ秒で指定し`cents`に各フレームの相対音高をセント単位で並べます。たとえば`"pitch_curve": {"frame_ms": 10, "cents": [0, 20, 40, 20, 0]}`です。GUI用の`manual_pitch`とは形式が異なります。

同じ文章で次の順に比較すると原因を絞り込めます。

1. 自動読みと修正した読みを比較する
2. 読みを固定して長さとピッチを調整する
3. 同じ読み・長さ・ピッチでRendererを変える

単語の聞き取り・音の欠落・接続・リズム・声質を別々に記録してください。ピーク・RMS・RTF・合成成功率は自然さの評価値ではありません。

## 英語・中国語の時間配分と抑揚を比較する

`--prosody-experiment`はCPU版WORLD専用の実験です。通常の合成には適用しません。`--model none`を指定しDelta・VCCV英語音源または中国語CVVC音源で比較します。

| 指定値 | 内容 |
| --- | --- |
| `baseline` | 従来の時間配分と抑揚。既定値 |
| `timing` | 句の合計時間を保って音節の時間を再配分 |
| `pitch` | 主音の原音から基準音高を決めて言語別の曲線を適用 |
| `both` | 時間配分と抑揚を両方変更 |

時間配分案は英語の強勢・弱母音・子音数と中国語の声調・軽声を使います。手動指定した長さは変更しません。抑揚案は原音ごとの音高差と共通の抑揚補正を外します。英語は最後の第一強勢を中心に音高を変えます。中国語はWORLDの母音開始時刻から声調を配置します。自動曲線の実験値であり自然さを保証するものではありません。

`english-prosody-v1.json`と`chinese-prosody-v1.json`は各3ケースです。英語では`--phonemizer en-delta`または`--phonemizer en-vccv`でコーパスの指定を上書きできます。

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/english-bank" --corpus tools/evaluation/english-prosody-v1.json --phonemizer en-delta --renderers utautts-world-phrase --model none --prosody-experiment timing --measure-pitch --out out/en-timing
```

4条件で同じコーパスと音源を使います。`report.json`に実験条件を保存します。音節長の配分を変えると先行発声や休止側へのはみ出しも変わるためWAV全体の長さがわずかに変わる場合があります。

`--measure-pitch`を付けると`*.pitch.json`へWORLDの目標F0と出力音声の推定F0を保存します。音声時刻の0 msはWAVの先頭です。Plan時刻には先行発声の余白を差し引きます。推定には既存のピッチ検出器を使い40 ms窓を10 msずつ動かします。

推定対象は60–500 Hzです。`measured_hz`の0は無声または推定不能を表します。`target_hz`はWORLDの有声・無声判定前の値です。誤差の集計は両方に有効な値があるフレームだけを使います。無声子音・短い母音・急な音高変化では推定が不安定になります。誤差が小さくても自然な発音とは限りません。計測時間はRTFに含めません。

## 選択した原音を確認する

`--export-sources`は音声ごとに`*-sources`フォルダーを作ります。

| ファイル | 内容 |
| --- | --- |
| `*-original-context.wav` | 選択範囲の前後150 msを含む原音 |
| `*-selected.wav` | 選択したoto範囲 |
| `*-mixed-output.wav` | 対応する合成音声の区間。隣接音との重なりを含む |
| `sources.json` | 原音のパス・切り出し範囲・母音開始の推定位置・出力時刻 |

原音の時刻は元WAVの先頭から測ります。出力時刻は先行発声の余白を含む合成WAVの先頭から測ります。これらの音声にはボイスバンクの原音が含まれます。共有する場合は音源の利用条件に従ってください。

### 単語境界のフェードを比較する

`--word-boundary-envelope`はCPU版WORLDで単語境界付近のフェード時間を半分にする実験です。5 ms以下のフェードは変えません。原音・切り出し範囲・音の配置・目標ピッチは変えず音量の立ち上がりと減衰だけを調整します。境界付近で複数の音が強く重なる場合もあるため自然さと音量の両方を試聴してください。

`english-word-boundary-v1.json`と`chinese-word-boundary-v1.json`でフラグの有無を比較できます。`--measure-pitch`で目標F0の一致を確認できます。Planのunitにある`boundary_envelope`は変更前後のフェード時間です。境界はfrontendの`WordIndex`に従います。中国語の自動分割は言語学的な単語境界を保証しません。休止をまたぐ箇所は変更しません。

### 英語の語末子音と次の母音を調べる

`english-cup-v1.json`はcup単体・cup of・Another cup of coffee.の比較です。ofの強勢ありとなしを読みで指定しています。無強勢化は原音候補に加えて規則による時間配分と抑揚にも影響します。実際に弱母音の原音が選ばれたかはPlanで確認してください。

### 英語の語末子音の再生範囲を確認する

CPU版WORLDではDelta・VCCV英語音源の必須語末子音について先行発声より後ろの原音を実際の再生終了までに収めます。固定部が長い原音で子音の後半より前に再生が終わる問題を補正します。元のoto値と音節長は変えません。補正は通常の合成で有効です。

新しい本体とWORLDブリッジが必要です。`english-cup-v1.json`を`--mora-ms 160`・`200`・`240`で比較できます。語末unitの`coda_phones`が必須の語末子音を示し`speech_retime_applied`で補正の適用を確認できます。任意のリリースや次の語頭だけを補う接続は対象外です。原音の後半には無音も含まれるため破裂部分が短くなる場合があります。

`english-coda-coverage-v1.json`は破裂音・摩擦音・鼻音・子音群と短文の16ケースです。`--phonemizer en-delta`または`en-vccv`で音源に合わせます。次の原音が必須語末子音の再生時間を10 ms未満まで削る場合は重なりを抑えます。この調整はCPU版WORLDに限定しています。原音に含まれない音は補えず音源や文章による聞き取りやすさの差は残ります。

## 合成速度を比較する

`--renderers utautts-world-phrase --repeat 2`で確認します。解析キャッシュを共有するため初回と2回目以降を分けて確認します。

RTFは合成時間を音声時間で割った値です。保存時間は含みません。文章の長さや音源によって速度が変わるため同じ条件で測定してください。

## 内部データと音源校正

frontendの`Phones`はalias表記から独立した発音単位です。中国語はPinyinの声母・韻母を使います。`Tone`には元の声調を保持して変調を音高曲線の生成時に適用します。声調曲線は韻母側へ配置します。

音源校正は実際に使う原音を遅延解析します。結果は音源インスタンスごとに最大4096件まで保持します。WAVのサイズ・更新時刻・oto値が変わると再計算し音源の再読込で破棄します。このキャッシュにはPCMを保持しません。
