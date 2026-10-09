# DiffSinger連携

DiffSingerは専用の音源とRendererで合成します。通常のUTAU音源とは異なり、`oto.ini`、resampler、wavtoolを使いません。

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

GUIでDiffSinger音源を選ぶと、DiffSinger Rendererへ自動で切り替わります。音節の長さとピッチは、GUIのプレビューと同じUtauTTSの抑揚モデルから作ります。手動の長さとピッチの編集にも対応します。抑揚を反映するには、ピッチ処理を有効にし、抑揚の強さを0より大きくします。

`dsdur`の予測は、音節全体の長さを保ったまま、子音と母音の配分にだけ弱く反映します。DiffSingerでは、子音、無声母音、句末に応じて話声向けのモーラ長も調整します。Open JTalkで単語境界を取得できる場合は、同じ単語の音素をまとめて渡します。ピッチは話声用の曲線を基準にし、`dspitch`に対応する音源では、音源側の細かなピッチの動きを弱く混ぜて急な変化を抑えます。手動で編集したモーラ長とピッチはそのまま使い、先頭の無音の分はピッチ曲線を配置するときに補正します。

UtauTTSは話声のタイミングと抑揚を反映し、音源に含まれる学習済みの音響モデルで音声を生成します。

合成では、bridgeの`utautts-provider`セッションを次の合成にも使い回します。C#のbridgeはモデルごとにONNX Runtimeの推論セッションを持ち続けるため、合成のたびにモデルを初期化しません。セッションを開始できない場合はエラーになります。

推論は`--diffsinger-steps`、`--diffsinger-duration-mix`、`--diffsinger-pitch-mix`、`--diffsinger-expr`（HTTPは`diffsinger_steps`など）で調整できます。いずれも0で既定値を使います。混合率を上げると音源側の予測が強くなり、下げるとUtauTTSの話声韻律を優先します。`--diffsinger-expr`は表現力で、既定の`0`はbridgeの既定値1.0を表します。下げるとビブラートや息遣いが弱まり、話し声に近づきます。
