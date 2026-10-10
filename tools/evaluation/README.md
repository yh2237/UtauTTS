# 読み上げ品質の評価

`tts-eval`は固定した文章と音源を使って、読み、原音候補、接続、合成音声を比較します。リポジトリのルートで実行し、[開発環境](../../docs/building.md)と評価対象のボイスバンクが必要です。

音源パスは評価対象のボイスバンクへ置き換えます。出力先には毎回新しいディレクトリを指定します。

## 読みと原音候補を診断する

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/english-bank" --corpus tools/evaluation/english-v1.json --diagnose --out out/english-diagnosis
go run ./cmd/tools/tts-eval --voicebank "./voice/chinese-bank" --corpus tools/evaluation/chinese-v1.json --diagnose --out out/chinese-diagnosis
```

英語と中国語のコーパスには、数字や多音字を含む自動読みと明示読みのケースがあります。各ケースの`language`と`phonemizer`を音源に合わせて比べるためのもので、発音全体を網羅してはいません。英語の既定値は`en-arpasing`です。Delta/VCCV音源では`en-delta`または`en-vccv`、C+V音源では`en-cv`を指定します。

`english-v2.json`は無強勢母音と単語をまたぐ子音群を含む8ケースです。`chinese-v2.json`は鼻音韻尾・üの表記差・変調を含む9ケースです。`--corpus`で指定します。v1は過去の比較用に残しています。中国語v2の鼻音ケースは`ban/bang`などの完全な音節を使います。v1の単独韻母の不足はv2とは別の結果として扱います。

`diagnostics.json`には読みと発音単位に加えて原音名（alias）の候補を保存します。診断モードでは原音WAVを読み込まず、音声も合成しません。音素化・候補探索の失敗や必須音の不足があれば、全ケースの処理後に終了コード1を返します。

| 項目 | 意味 |
| --- | --- |
| `coverage.positions` | 休止を除く位置数 |
| `coverage.covered` | 有効な主候補がある位置数 |
| `coverage.candidate_counts` | 候補絞り込み後の件数。添字は休止を含む発音単位に対応 |
| `coverage.missing` | 主候補の不足位置と探索したaliasや除外理由 |
| `coverage.missing_phones` | 最も不足が少ない候補でも欠ける必須音 |
| `lattice` | 経路選択結果。全文の探索失敗時は保存なし |

促音の無音区間も有効候補に含みます。候補数と発音の正しさは別指標です。原音の収録内容と接続は聴取で評価します。

`coverage_summary.json`には言語別の集計を保存します。`coverage_rate`は有効な主候補がある位置の割合、`missing_phone_rate`は必須音が欠けた位置の割合、`missing_phones`は欠けた必須音の頻度、`missing_morae`は主候補が無かったモーラの頻度です。音源と言語の組み合わせの網羅性を比べる目安にします。

## 音声を比較する

```powershell
go run ./cmd/tools/tts-eval --voicebank "./voice/japanese-bank" --renderers utautts-world-phrase --model none --repeat 1 --out out/ja-base
```

コーパスの既定値は`tools/evaluation/japanese-v1.json`の8文です。`--model none`は学習済み抑揚モデルを使いません。任意のモデルは`--model-file`でJSONのパスを指定できます。

英語・中国語でも`--corpus`を指定して比較できます。`--diagnose`を付けると診断、外すと音声の比較です。ビルドしたブリッジは`--bridge`で指定できます。

WAV・TXT・LAB・合成計画（Plan JSON）・`report.json`を出力します。合成計画の時刻は生成時の目標値で、音声から実測した境界ではありません。

| 診断項目 | 意味 |
| --- | --- |
| Planの`phone_timings` | 発音単位ごとの目標時刻 |
| Planの`missing_phones` | 選択した経路で不足した必須語尾や検出可能な句頭子音群 |
| unitの`speech_profile` | 固定部の元の値と提案値。安定区間・F0・音量・有声率・信頼度・採否の理由 |
| unitの`speech_retime_applied` | CPU版WORLDの区間別伸縮を適用したか |
| unitの`speech_join_applied` | CPU版WORLDの母音接続補修を適用したか |
| unitの`effective_consonant_ms` | 伸縮後の固定部の位置 |
| Planの`boundary_repair_decisions` | 接続補修の候補数と採否。補修前後の波形差分指標 |
| reportの`missing_phone_groups` | 必須音が不足したグループの件数 |

任意のリリース音は必須音の不足と区別します。語中の複雑な子音群や原音の発音誤りは一部しか検出できません。音声生成では代替候補を使うため、生成の成功と音の欠落は別に評価します。

子音と接続の比較には`--corpus tools/evaluation/japanese-connection-v1.json`を指定します。同じ読みを使う6ケースで母音連続・破裂音・摩擦音・鼻音・長母音を確認できます。単独音・連続音など収録形式の違う音源で比べます。

出力するPlan JSONには合成後の診断情報を含みます。区間別伸縮を適用しても指定したモーラ長は変えません。`boundary_repair_decisions`の指標は補修箇所の比較用で、自然さは試聴で評価します。

解析キャッシュを使う2回目の合成は`--repeat 2`で比較できます。

## 読み・長さ・ピッチを固定する

コーパスはケースの配列です。`reading`で読みを固定できます。`mora_durations_ms`はミリ秒単位の長さ配列です。配列の要素数は診断結果の発音単位数に合わせてください。単位はphonemizerによって異なります。

`pitch_curve`は一定間隔のピッチ列です。`frame_ms`にフレーム間隔をミリ秒で指定し、`cents`に各フレームの相対音高をセント単位で並べます。たとえば`"pitch_curve": {"frame_ms": 10, "cents": [0, 20, 40, 20, 0]}`です。GUI用の`manual_pitch`とは形式が異なります。

同じ文章で次の順に比較すると原因を絞り込めます。

1. 自動読みと修正した読みを比較する
2. 読みを固定して長さとピッチを調整する
3. 同じ読み・長さ・ピッチでRendererを変える

聴取では単語の聞き取り、音の欠落、接続、リズム、声質を確認します。peak、RMS、RTF、合成成功率は性能の目安で、自然さは聴取で評価します。

## 目標F0と出力F0を比べる

`--measure-pitch`を付けると`*.pitch.json`へWORLDの目標F0と出力音声の推定F0を保存します。音声時刻の0 msはWAVの先頭です。Plan時刻には先行発声の余白を差し引きます。推定には既存のピッチ検出器を使い、40 ms窓を10 msずつ動かします。

推定対象は60–500 Hzです。`measured_hz`の0は無声または推定不能を表します。`target_hz`はWORLDの有声・無声判定前の値です。誤差の集計は両方に有効な値があるフレームだけを使います。無声子音・短い母音・急な音高変化では推定が不安定になります。誤差と自然な発音は別評価です。計測時間はRTFに含めません。

## 選択した原音を確認する

`--export-sources`は音声ごとに`*-sources`フォルダーを作ります。

| ファイル | 内容 |
| --- | --- |
| `*-original-context.wav` | 選択範囲の前後150 msを含む原音 |
| `*-selected.wav` | 選択したoto範囲 |
| `*-mixed-output.wav` | 対応する合成音声の区間。隣接音との重なりを含む |
| `sources.json` | 原音のパス・切り出し範囲・母音開始の推定位置・出力時刻 |

原音の時刻は元WAVの先頭から測ります。出力時刻は先行発声の余白を含む合成WAVの先頭から測ります。これらの音声にはボイスバンクの原音が含まれます。共有する場合は音源の利用条件に従ってください。

## 合成速度を比較する

`--renderers utautts-world-phrase --repeat 2`で確認します。解析キャッシュを共有するため、初回と2回目以降を分けて確認します。

RTFは合成時間を音声時間で割った値で、保存時間は含みません。文章と音源をそろえて比べてください。

## 内部データと音源校正

frontendの`Phones`はalias表記から独立した発音単位です。中国語はPinyinの声母・韻母を使います。`Tone`には元の声調を保持し、変調は音高曲線の生成時に適用します。声調曲線は韻母側へ配置します。

音源校正は実際に使う原音を遅延解析します。結果は音源インスタンスごとに最大4096件まで保持します。WAVのサイズ・更新時刻・oto値が変わると再計算し、音源の再読込で破棄します。キャッシュするのは解析結果だけです。
