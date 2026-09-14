# JSUT音素データと接続事前分布

JSUT BASIC5000のラベルから音素単位の時間情報と音響観測値を作り、UTAU音源へ適応する前の事前分布を生成できます。用途: 開発・評価。GUIやRendererで使う場合は明示指定します。

## データの準備

入力データ:

- JSUT BASIC5000の`wav/`と`transcript_utf8.txt`
- `sarulab-speech/jsut-label`のBASIC5000 HTSラベル

利用条件: [JSUTとラベルの通知](../licenses/JSUT-DATA-AND-LABELS.txt)。入力データはローカルで扱い、配布物にはモデルJSONとライセンス通知を収録します。

## 音素データを作る

```powershell
go run ./cmd/tools/jsut-join-dataset --labels "./data/jsut-label" --corpus "./data/jsut/basic5000" --out "./out/jsut-join.jsonl"
```

出力は1行1発話のJSONLです。各発話に次の情報を保存します。

- HTSラベルの音素ごとの開始・終了時刻
- 前後の音素と元のコンテキストラベル
- アクセント句の位置・長さ・核
- 音素中央のRMS・F0・スペクトル観測値
- 音素境界のスペクトル差・レベル差・F0差・有声無声差

自然な音素接続の観測値: `join_type`が`phone`の境界。`silence`と`pause`: 発話区切り。学習用の接続正例: `phone`のみ。`phone`の`label`は1、それ以外の`label`は`null`です。時刻の推定元: `alignment_source`に示すJulius。

WAVを読み込まず時刻だけ作る場合は`--skip-audio-features`を付けます。短い音素や無声音ではフレームが取得できないことがあります。その場合も音素時刻は保持され、境界の`features`だけが省略されます。

## JSUT事前分布を作る

```powershell
go run ./cmd/tools/jsut-target-prior --input "./out/jsut-join.jsonl" --out "./out/jsut-target-prior.json"
```

出力JSONには音素単位と前後音素を含むコンテキスト単位の次の統計が入ります。

- 音素継続時間
- 音素中央のRMS・F0・スペクトル
- 有声率
- 音素ペアの自然境界で観測された差分

JSONには生成物の配布条件を示す`license`・`license_notice`・`data_notice`も保存します。

## ホールドアウト評価

発話IDのハッシュで学習用と評価用を分け、音素継続時間の平均絶対誤差と二乗平均平方根誤差を測ります。コンテキスト統計を使わない音素単位の予測と比較できます。`mora_phone_allocation`ではモーラの合計時間を実測値に固定し、現在の`PhoneSpans`と音素・コンテキスト事前分布の子音・母音への配分を比較します。これは現在の固定重みが実際の音素境界に合っているかを確認する指標です。

```powershell
go run ./cmd/tools/jsut-target-eval --input "./out/jsut-join.jsonl" --out "./out/jsut-target-eval.json"
```

`context_mae_gain_ms`が正なら、前後音素の文脈を加えた予測の誤差が小さくなっています。評価対象は音素の時間配置です。合成音声の自然さと話者一般化は別途評価します。

BASIC5000全5000発話を80%学習・20%評価に分けた実測値は次のとおりです。

- 音素継続時間のMAE: 全体平均24.12 ms、音素平均19.94 ms、コンテキスト付き13.75 ms
- モーラ内の音素配分MAE: 現行`PhoneSpans`30.53 ms、コンテキスト付き7.94 ms
- 評価したモーラ内音素は61,303個、モーラは34,976個

この差はJSUT話者の時間配置に対する結果です。UTAU音源への適応結果は音源ごとの比較音声で評価します。

このモデルはJSUT話者の自然な目標軌跡を表す初期値です。UTAU音源の声質や`oto.ini`の切り出し位置は音源側の情報として扱います。Rendererでは明示指定時に適用します。

実音源で試す場合はCLIで明示的に指定します。

```powershell
go run ./cmd/utautts-cli --voicebank "./sample/uta" --text "これは実音源での評価です。" --renderer utautts-world-phrase --target-prior "./out/jsut-target-prior.json" --out "./out/target-prior.wav"
```

`--target-prior`はモーラ長を変えず、音素の時間配分と子音境界を調整します。`--target-prior-strength 0.5`のように指定すると標準の固定配分と混ぜられます。連続音で試す場合は`--speech-timing`も指定します。用途: 検証。GUIやRendererの既定値: 維持。

## UTAUへ適応する流れ

UTAU固有の接続品質は既存の監査データから別に学習します。

```powershell
go run ./cmd/utautts-cli --voicebank "./sample/音源" --reading "これはテストです。" --out "./out/audit.wav" --plan-out "./out/audit.plan.json"
go run ./cmd/tools/join-audit --plan "./out/audit.plan.json" --out "./out/audit.json"
go run ./cmd/tools/join-ranker --input "./out/audit.json" --out "./out/utau-join-ranker.json"
```

JSUTの自然境界は自動生成の正例です。UTAUの良否ラベルは、音源・文・候補の聴取結果から別途作成します。`join-ranker`の入力: UTAUの接続データ。構成: JSUT事前分布を候補の目標コスト、UTAUの接続モデルを境界コストに使用します。

実行時の依存: Go標準ライブラリと既存の音響解析コード。
