# DiffSinger（試験実装）

DiffSingerは専用の音源とRendererで合成する。`oto.ini`、resampler、wavtoolは使わない。

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

共有vocoderは`Dependencies/<名前>`に置く。名前は音源の指定と一致させる。UtauTTSの実行ファイルまたは作業ディレクトリにある`Dependencies`を探す。OpenUtauの標準`Dependencies`も利用できる。

## 未対応

- 話者混合
- GPUとWindows以外の実行環境

GUIではDiffSinger音源を選ぶと、DiffSinger Rendererへ自動で切り替わる。
歌唱感を抑えるため、音素長とピッチはUtauTTSの話声設定を主体にしてモデルの予測を弱く混ぜる。
