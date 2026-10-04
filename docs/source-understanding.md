# 原音区間ライブラリ

原音内の音素区間を合成側の音素時刻へ対応付けるための、開発者向けの解析・導入手順です。通常の読み上げについては[日本語・英語・中国語の読み上げ](multilingual.md)を参照してください。

合成処理全体の担当と補正順は[UTAU音源の合成処理](synthesis-architecture.md)にあります。

観測ツールにはGoとPython、図の表示にはNumPyとMatplotlibが必要です。強制整列を行う場合はMFAと対象言語の音響モデルも用意してください。

## 通常の合成で使う

CPU版WORLDの英語・中国語では、登録済みの原音区間を自動で使用します。ボイスバンク直下の`source-phone-library.json`を優先し、次にモデル探索先の`source-phones/en.json`または`source-phones/zh.json`を参照します。同梱データの対象は[原音区間ライブラリの収録範囲](../models/source-phones/README.md)にあります。

原音のハッシュと音素列が一致する場合だけ適用し、未知・曖昧・不整合な区間は既存のoto写像を使います。合成中にMFAを起動したりモデルをダウンロードしたりすることはありません。

HTTP APIでは次のWORLD設定で自動適用を無効にできます。

```json
{"worldline": {"source_phone_mapping": false}}
```

## 原音を観測する

描画後の合成計画を`--plan-out`で保存してから実行します。

```powershell
go build -o out/source-analyze.exe ./cmd/tools/source-analyze
out/source-analyze.exe --plan out/utterance.plan.json --out out/source-observation
```

`--positions 2,7,9`で発音単位を絞れます。出力は`observations.json`と、otoで切り出した原音WAVです。元の音源や合成設定は変更しません。

| 項目 | 内容 |
| --- | --- |
| `requested_context` | 合成側が要求する音素と目標時刻 |
| `assigned_coda_phones` | 原音へ割り当てた語末音素 |
| `analysis` | RMS・ピーク・ゼロ交差率・周期性と境界候補 |
| `forced_phone_intervals` | 強制整列から取り込んだ音素区間 |

観測特徴や境界候補は音素認識の結果ではありません。強制整列の区間も、録音内容と対応しているか確認が必要です。

## 音素列を指定して整列する

録音に含まれる音素を、MFA音響モデルの記号で指定します。次は英語ARPAbetの例です。

```json
{"units": [{"unit_index": 6, "phones": ["L", "D"]}]}
```

```powershell
python tools/source-phone-alignment.py prepare --report out/source-observation/observations.json --requests out/source-requests.json --out out/source-alignment
mfa align out/source-alignment/corpus out/source-alignment/dictionary.dict english_us_arpa out/source-alignment/alignments --output_format json --num_jobs 1 --no_use_postgres --single_speaker
python tools/source-phone-alignment.py import --manifest out/source-alignment/manifest.json --alignments out/source-alignment/alignments --model english_us_arpa --out out/source-alignment/aligned-observations.json
```

音素列の不一致、未知音素、重複・範囲外の時刻は取り込みません。`alignment_audit`に成功と除外理由を記録します。音響モデルとUtauTTSの音素表記が異なる場合は、入力の`canonical_phones`に同じ順序の内部記号を指定できます。

エイリアスから候補列を作る場合は`source-phone-discovery.py`（英語）または`source-phone-discovery-zh.py`（中国語）を使います。各ツールの`prepare --help`と`finish --help`で引数を確認できます。中国語の生成対象は単母音と鼻音韻尾を持つ音節で、複合母音は対象外です。生成した`mfa-config.yaml`をMFAの`--config_path`へ指定してください。

## 区間を確認する

```powershell
python tools/source-phone-alignment.py audit --report out/source-alignment/aligned-observations.json --out out/source-alignment/acoustic-audit.json
python tools/plot-source-analysis.py --report out/source-alignment/aligned-observations.json --alias "l d-" --detail --out out/source-alignment/detail.png
```

原音の波形・スペクトログラム・RMSに整列区間を重ねて確認できます。`audit`は整列外の音響活動や破裂音の境界候補を調べます。`needs-review`は確認が必要な区間です。警告がない場合も、音素境界の正確さを保証するものではありません。

人手で確認した境界と比較する場合は、参照JSONの各unitに`unit_index`・`source_sha256`・`annotation_kind: manual`を記録し、`phones`へ音素ごとの`symbol`・`start_ms`・`end_ms`を指定します。

```powershell
python tools/source-phone-alignment.py evaluate --report out/source-alignment/aligned-observations.json --references data/manual-source-phones.json --out out/source-alignment/boundary-metrics.json
```

音素開始・終了のMAE・最大誤差・20ms以内の割合を出力します。未注釈の原音は精度計算に含めません。

## ライブラリを作る

```powershell
python tools/source-span-mapping.py propose --report out/source-alignment/aligned-observations.json --out out/span-requests.json
python tools/source-span-mapping.py select --report out/source-alignment/aligned-observations.json --requests out/span-requests.json --out out/source-spans
go run ./cmd/tools/source-span-auto build --spans out/source-spans/spans.json --out out/source-phone-library.json
```

英語は担当する語末音素、中国語は鼻音韻尾を含む音節全体を対応付けます。対応が一意でない区間は候補から除外します。`select`は`spans.json`と対象区間・隣接区間の確認用WAVを出力します。既存結果を上書きしないため、新しい出力ディレクトリを使ってください。

複数の区間ファイルは`build`へ`--spans`を繰り返し指定できます。同じ原音に異なる仮説がある場合はエラーになります。`--prefer-last`を明示した場合のみ後の候補を優先し、置換内容を記録します。`source-span-auto.py map`はMFA監査とPythonの`source-span-mapping.py`を呼ぶため、整列ワークフローに残します。

確認したライブラリをボイスバンク直下の`source-phone-library.json`へ配置します。原音WAVはライブラリに含まれません。強制整列の結果を、そのまま人手確認済みの教師データとして使用しないでください。

## 時刻と原音の識別

原音時刻はoto.offsetを起点とし、終端はoto.cutoffの規約に従います。元WAV全体の時刻ではありません。

`source_sha256`は切り出した16bit PCMとサンプルレート・チャンネル数から計算します。元ファイル全体のハッシュではありません。ライブラリの`version`は1、`time_origin`は`oto-offset`、`language`は`en`または`zh`です。`entries`にハッシュ・`duration_ms`・`phones`を記録し、各音素は`symbol`・`start_ms`・`end_ms`を持ちます。

合成計画の`phone_timings`は文章内の目標音素時刻です。`speech_source_anchors_ms`は原音時刻、`speech_target_anchors_ms`はその原音の出力開始からの時刻を表します。ライブラリを適用した区間は`speech_mapping: source-phone-library-v1`として記録します。
