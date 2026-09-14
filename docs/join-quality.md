# 接続品質の監査と学習

音源の接続境界を確認し、必要な場合に接続候補の評価モデルを作成します。

UtauTTSには、音源の接続境界を調べる監査ツールと、聴取ラベルから接続スコアを学習するモデルがあります。接続モデルを指定しない合成では、標準の評価を使います。

## 監査データを作る

まず合成計画を出力します。

```powershell
go run ./cmd/utautts-cli --voicebank "./sample/重音テト OU用日本語統合ライブラリー" --reading "これはテストです。" --out out/join-audit.wav --plan-out out/join-audit.plan.json
```

計画内の隣接した音源を分析します。出力の`label`はまだ`null`です。

```powershell
go run ./cmd/tools/join-audit --plan out/join-audit.plan.json --out out/join-audit.json
```

`risk_flags`: 聴取する境界を絞るための目印。`label`: 学習ラベル。入力値:

* `1` 接続が自然で、境界を採用したい
* `0` 段差、ノイズ、切り落としなどがあり採用したくない

音源や文ごとに複数の監査ファイルを作り、同じ形式でラベルを付けます。少なくとも正例と負例を4行以上用意します。

## モデルを学習する

学習器の実行環境: Go標準ライブラリと既存の音響解析コード。

```powershell
go run ./cmd/tools/join-ranker --input out/join-audit.json --out out/join-ranker.json
```

複数ファイルは`--input`を繰り返して指定します。出力JSONには特徴量の順序、正規化値、学習条件、ラベルの意味を保存します。ボイスバンク録音の共有モデル利用条件: 各ライセンスと作者の許諾。

## 合成で使う

CLIでは`--join-model`を指定します。

```powershell
go run ./cmd/utautts-cli --voicebank "./sample/重音テト OU用日本語統合ライブラリー" --reading "これはテストです。" --join-model out/join-ranker.json --out out/join-learned.wav --plan-out out/join-learned.plan.json
```

モデルは現在の手作りスコアへ限定的な補正を加えます。確信度が低い境界では補正せず、手作りスコアへ戻ります。計画には`join_cost_mode: "learned"`と`join_model_id`が記録されます。

モデルの対象: 候補を選ぶ接続コスト。波形とWORLD特徴: 変更なし。音源不足、`oto.ini`で切り落とされた子音、録音ノイズ: モデルの対象外。
