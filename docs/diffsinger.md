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

共有vocoderは`Dependencies/<名前>`に配置します。名前は音源の指定と一致させてください。UtauTTSの実行ファイルまたは作業ディレクトリにある`Dependencies`を検索します。OpenUtauの標準`Dependencies`も利用できます。

## 制限事項

- 話者混合
- GPUを使った実行
- Windows以外の実行環境

GUIでDiffSinger音源を選ぶと、DiffSinger Rendererへ自動で切り替わります。音節の長さとピッチは、GUIのプレビューと同じUtauTTSの抑揚モデルから作ります。手動の長さとピッチ編集にも対応します。抑揚の適用条件: ピッチ処理を有効化し、抑揚の強さを0より大きく設定します。

`dsdur`は音節全体の長さを保ったまま、子音と母音の配分へ弱く反映します。話声用のピッチ曲線がある場合は`dspitch`を使わず、その曲線を合成へ渡します。先頭の無音分はピッチ曲線の配置時に補正します。

処理の役割: 話声のタイミングと抑揚の反映。音響モデル: 音源に含まれる学習済みモデル。音源によっては歌唱由来の発声が残ります。

合成時は、対応するbridgeの`utautts-provider` sessionを同じプロセスで次の合成にも再利用します。C# bridgeはモデルごとにONNX Runtimeの推論sessionを保持するため、合成ごとのモデル初期化を避けられます。sessionを開始できない場合はエラーになります。
