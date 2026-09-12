# 接続品質の監査と学習

UtauTTSには、音源の接続境界を調べる監査ツールと、聴取ラベルから接続スコアを学習する小さなモデルがあります。モデルを指定しない合成は従来どおりです。

## 監査データを作る

まず合成計画を出力します。

```powershell
go run ./cmd/utautts-cli --voicebank "./sample/重音テト OU用日本語統合ライブラリー" --reading "これはテストです。" --out out/join-audit.wav --plan-out out/join-audit.plan.json
```

計画内の隣接した音源を分析します。出力の`label`はまだ`null`です。

```powershell
go run ./cmd/tools/join-audit --plan out/join-audit.plan.json --out out/join-audit.json
```

`risk_flags`は聴取する境界を絞るための目印です。学習ラベルではありません。`label`へ次の値を入力します。

* `1` 接続が自然で、境界を採用したい
* `0` 段差、ノイズ、切り落としなどがあり採用したくない

音源や文ごとに複数の監査ファイルを作り、同じ形式でラベルを付けます。少なくとも正例と負例を4行以上用意します。

## モデルを学習する

学習器はGoだけで動作し、PythonやONNX Runtimeを必要としません。

```powershell
go run ./cmd/tools/join-ranker --input out/join-audit.json --out out/join-ranker.json
```

複数ファイルをまとめる場合は`--input`を繰り返します。出力JSONには特徴量の順序、正規化値、学習条件、ラベルの意味を保存します。ボイスバンクの録音を共有モデルの学習に使えるかは、各ライセンスと作者の許諾を確認してください。

## 合成で使う

CLIでは`--join-model`を指定します。

```powershell
go run ./cmd/utautts-cli --voicebank "./sample/重音テト OU用日本語統合ライブラリー" --reading "これはテストです。" --join-model out/join-ranker.json --out out/join-learned.wav --plan-out out/join-learned.plan.json
```

モデルは現在の手作りスコアへ限定的な補正を加えます。確信度が低い境界では補正せず、手作りスコアへ戻ります。計画には`join_cost_mode: "learned"`と`join_model_id`が記録されます。

モデルは候補を選ぶ接続コストだけに使い、波形やWORLD特徴は変更しません。音源不足、`oto.ini`で切り落とされた子音、録音そのもののノイズはこのモデルでは復元できません。
