# DiffSinger連携

DiffSingerは専用の音源とRendererで合成します。通常のUTAU音源とは異なり、`oto.ini`、resampler、wavtoolを使わない経路です。対応範囲と配置を次に示します。

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

## 制限事項

- 話者混合
- GPUを使った実行
- Windows以外の実行環境

GUIでDiffSinger音源を選ぶと、DiffSinger Rendererへ自動で切り替わります。音節の長さとピッチは、GUIのプレビューと同じUtauTTSの抑揚モデルから作ります。手動の長さとピッチ編集にも対応します。抑揚の適用条件: ピッチ処理を有効化し、抑揚の強さを0より大きく設定します。

`dsdur`は音節全体の長さを保ったまま、子音と母音の配分へ弱く反映します。DiffSingerでは子音、無声母音、句末に応じて話声向けのモーラ長も調整します。Open JTalkで単語境界を取得できる場合は、同じ単語の音素をまとめて渡します。話声用のピッチ曲線がある場合は、その曲線を基準にします。`dspitch`に対応する音源では、音源側の微小なピッチ変化を弱く混ぜて急な変化を抑えます。手動のモーラ長とピッチ編集はそのまま使います。先頭の無音分はピッチ曲線の配置時に補正します。

UtauTTSは話声のタイミングと抑揚を反映し、音源に含まれる学習済みの音響モデルで音声を生成します。

合成時は、対応するbridgeの`utautts-provider` sessionを同じプロセスで次の合成にも再利用します。C# bridgeはモデルごとにONNX Runtimeの推論sessionを保持するため、合成ごとのモデル初期化を避けられます。sessionを開始できない場合はエラーになります。

推論は`--diffsinger-steps`、`--diffsinger-duration-mix`、`--diffsinger-pitch-mix`、`--diffsinger-expr`（HTTPは`diffsinger_steps`など）で調整できます。いずれも0で既定値を使います。混合率を上げると音源側の予測が強くなり、下げるとUtauTTSの話声韻律を優先します。`--diffsinger-expr`はフラグの既定`0`がbridgeの既定値1.0を意味する表現力で、下げるとビブラートや息遣いが弱まり話声寄りになります。
