# 原音区間ライブラリ

原音の音素区間（母音・子音・語末子音境界）を解析し、多言語合成用のライブラリとして登録する手順です。

※CPU版WORLDの英語・中国語では登録済みの原音区間を自動で使用するため、通常は本手順を実施する必要はありません。自作音源や未登録音源に対応させる開発者向けの手順です。

ボイスバンク直下の `source-phone-library.json` を優先し、次にモデル探索先の `source-phones/en.json` または `source-phones/zh.json` を参照します。[同梱データの収録範囲](../models/source-phones/README.md)も確認してください。

---

## 1. 原音の音響観測

合成計画を出力後、ツールで原音の音響的特徴（RMS・ゼロ交差率・周期性）を抽出します。

```powershell
go run ./cmd/tools/source-analyze --plan out/utterance.plan.json --out out/source-observation
```

`observations.json` および切り出された各原音WAVが出力されます。
---

## 2. MFA（Montreal Forced Aligner）による音素区間の整列

録音に含まれる音素列を定義し、MFAを用いて音素境界を強制整列します（以下は英語ARPAbetの例）。

```powershell
# 1. 整列用コーパスの準備
go run ./cmd/tools/source-phone-alignment prepare `
  --report out/source-observation/observations.json `
  --requests out/source-requests.json `
  --out out/source-alignment

# 2. MFAの実行
mfa align out/source-alignment/corpus out/source-alignment/dictionary.dict english_us_arpa out/source-alignment/alignments --output_format json --num_jobs 1 --no_use_postgres --single_speaker

# 3. 整列結果の取り込み
go run ./cmd/tools/source-phone-alignment import `
  --manifest out/source-alignment/manifest.json `
  --alignments out/source-alignment/alignments `
  --model english_us_arpa `
  --out out/source-alignment/aligned-observations.json
```

---

## 3. 区間ライブラリのビルドと配置

整列結果から一意に定まる音素区間を選択し、ライブラリJSONをビルドします。

```powershell
go run ./cmd/tools/source-span-mapping propose --report out/source-alignment/aligned-observations.json --out out/span-requests.json
go run ./cmd/tools/source-span-mapping select --report out/source-alignment/aligned-observations.json --requests out/span-requests.json --out out/source-spans
go run ./cmd/tools/source-span-auto build --spans out/source-spans/spans.json --out out/source-phone-library.json
```

区間を確認した後、生成された `source-phone-library.json` をボイスバンクのフォルダ直下へ配置します。
