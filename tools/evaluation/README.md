# 読み上げ品質の評価

`tts-eval`は固定した文章で読み・原音選択・合成音声を比較する開発者向けツールです。リポジトリのルートから実行します。[開発環境](../../docs/building.md)と評価対象のボイスバンクを用意してください。

以下の音源パスは手元のボイスバンクに置き換えてください。出力先には毎回新しいディレクトリを指定します。

## 読みと原音候補を診断する

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/english-bank" --corpus tools/evaluation/english-v1.json --diagnose --out out/english-diagnosis
go run ./cmd/tools/tts-eval --voicebank "./voice/chinese-bank" --corpus tools/evaluation/chinese-v1.json --diagnose --out out/chinese-diagnosis
```

英語と中国語のコーパスには自動読みと明示読みのケースがあります。数字や多音字も含みますが発音を網羅する評価セットではありません。各ケースの`language`と`phonemizer`を音源に合わせてください。英語の既定値は`en-arpasing`です。Delta/VCCV音源では`en-delta`または`en-vccv`を指定します。

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

CPU版WORLDは音響特徴の時間軸と母音接続を調整します。`--renderers utautts-world-phrase --speech-timing`で比較できます。本体とブリッジの両方をビルドしてください。解析キャッシュを使う2回目の合成も確認する場合は`--repeat 2`を指定します。CUDA版はこの区間別伸縮と接続補修の対象外です。

## 読み・長さ・ピッチを固定する

コーパスはケースの配列です。`reading`で読みを固定できます。`mora_durations_ms`はミリ秒単位の長さ配列です。配列の要素数は診断結果の発音単位数に合わせてください。単位はphonemizerによって異なります。

`pitch_curve`は一定間隔のピッチ列です。`frame_ms`にフレーム間隔をミリ秒で指定し`cents`に各フレームの相対音高をセント単位で並べます。たとえば`"pitch_curve": {"frame_ms": 10, "cents": [0, 20, 40, 20, 0]}`です。GUI用の`manual_pitch`とは形式が異なります。

同じ文章で次の順に比較すると原因を絞り込めます。

1. 自動読みと修正した読みを比較する
2. 読みを固定して長さとピッチを調整する
3. 同じ読み・長さ・ピッチでRendererを変える

単語の聞き取り・音の欠落・接続・リズム・声質を別々に記録してください。ピーク・RMS・RTF・合成成功率は自然さの評価値ではありません。

## CPUとCUDAの速度を比較する

`--renderers utautts-world-phrase,utautts-world-phrase-cuda --repeat 2`で比較します。CUDA対応ランタイムが必要です。解析キャッシュを共有するため各Rendererの初回と2回目以降を分けて確認します。

RTFは合成時間を音声時間で割った値です。保存時間は含みません。CUDA化の対象は特徴量混合で波形生成はCPUです。文章の長さや音源によって速度が変わるため同じ条件で測定してください。

## 内部データと音源校正

frontendの`Phones`はalias表記から独立した発音単位です。中国語はPinyinの声母・韻母を使います。`Tone`には元の声調を保持して変調を音高曲線の生成時に適用します。声調曲線は韻母側へ配置します。

音源校正は実際に使う原音を遅延解析します。結果は音源インスタンスごとに最大4096件まで保持します。WAVのサイズ・更新時刻・oto値が変わると再計算し音源の再読込で破棄します。このキャッシュにはPCMを保持しません。
