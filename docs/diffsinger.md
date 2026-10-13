# DiffSinger連携

専用のDiffSinger音源を選んで合成します。`oto.ini`、resampler、wavtoolは使いません。

## 対応範囲

- OpenUtau形式の`dsconfig.yaml`
- テキスト形式とJSON形式の音素表
- 音源内の`dsvocoder`とOpenUtau形式の共有vocoder
- 音源パッケージ内の共有モデル
- 言語ID、話者埋め込み、gender、velocity
- `dsdur`による音素長配分
- `dspitch`によるピッチ推定
- `dsvariance`による声質推定
- 連続・離散diffusion
- 日本語かな入力

共有vocoderは`Dependencies/<名前>`に配置します。名前は音源の指定と一致させてください。探索先は、UtauTTSを起動したディレクトリの`Dependencies`、実行ファイルと同じ場所の`Dependencies`、OpenUtauの標準`Dependencies`の順です。

## 対応していないもの

- 話者混合
- GPUを使った実行
- Windows以外の実行環境

## 合成と調整

GUIでDiffSinger音源を選ぶとRendererも切り替わります。長さとピッチは通常のエディターで調整し、手動で指定した値を優先します。抑揚を使うにはピッチ処理を有効にし、抑揚の強さを0より大きくします。

CLIでは次のオプション、HTTPでは`diffsinger_steps`などで調整します。いずれも0で既定値を使います。

| オプション | 効果 |
| --- | --- |
| `--diffsinger-steps` | 拡散ステップ数を変更 |
| `--diffsinger-duration-mix` | 音節内の子音・母音の配分を調整。上げると音源側、下げるとUtauTTSの予測を優先 |
| `--diffsinger-pitch-mix` | 上げると音源側のピッチ予測、下げるとUtauTTSの話声抑揚を優先 |
| `--diffsinger-expr` | 下げるとビブラートや息遣いが弱まる。0は既定値1.0 |
