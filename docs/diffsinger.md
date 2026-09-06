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
音節の長さとピッチは、GUIのプレビューと同じUtauTTSの抑揚モデルから作る。手動の長さ・ピッチ編集にも対応する。抑揚を使う場合はピッチ処理を有効にし、抑揚の強さを0より大きく設定する。

`dsdur`は音節全体の長さを保ったまま子音・母音の配分へ弱く反映する。話声用のピッチ曲線がある場合は`dspitch`を使わず、その曲線を合成へ渡す。先頭の無音分はピッチ曲線の配置時に補正する。

これらは話声のタイミングと抑揚を反映する処理であり、音響モデル自体を話声用に再学習するものではない。音源によっては歌唱由来の発声が残る。

合成時は、対応するbridgeが`utautts-provider` session modeを実装していれば同じプロセスを次の合成でも再利用する。C# bridgeはモデルpathごとにONNX Runtimeの推論sessionを保持するため、合成ごとのモデル初期化を避けられる。古いbridgeを指定した場合は従来の1回起動経路へ自動的にfallbackする。
