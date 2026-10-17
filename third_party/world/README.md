# WORLD

このディレクトリには公式WORLDボコーダーのソーススナップショットを収録しています。解析を速くするため、UtauTTSで次の変更を加えています。

- `src/fft.cpp`: FFTの計画を作るたびに計算していた表を、スレッドごとに一度だけ作って写します。
- `src/harvest.cpp`: F0の候補を求め直すとき、同じフレームで窓が同じ候補はスペクトルを使い回します。
- `src/harvest.cpp`、`src/cheaptrick.cpp`、`src/d4c.cpp`: 窓とリフタのcosとsinを、加法定理で順に求めます。
- `src/d4c.cpp`: 非周期性を求めるとき、パワースペクトルをすべて並べ替えずに、小さいほうの値だけを選びます。

最初の2つは結果を変えません。残りの2つは丸め誤差の範囲で結果が変わります（スペクトルの相対差は3.5e-8以下）。

- 上流プロジェクト: https://github.com/mmorise/World
- 取得コミット: `d625e7608ca23a870018f01e7c562ac683d9847f`
- ライセンス: modified BSD（`LICENSE.txt`）

FFT実装の上流許諾は`OOURA-NOTICE.txt`に、`src/world/macrodefinitions.h`が含むMIT通知は`MACRODEFINITIONS-LICENSE.txt`に記載しています。

UtauTTSでは、このソースを独自の解析・合成ライブラリとしてビルドします。
