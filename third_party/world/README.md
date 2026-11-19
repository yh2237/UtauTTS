# WORLD

このディレクトリには公式WORLDボコーダーのソーススナップショットを収録しています。解析を高速化するため、UtauTTSで次の変更を加えています。

- `src/fft.cpp`: FFT計画の作成ごとに計算していたテーブルを、スレッドごとに1回だけ生成してコピーします。
- `src/harvest.cpp`: F0候補を再計算するとき、同じフレームで窓が同じ候補のスペクトルを使い回します。
- `src/harvest.cpp`、`src/cheaptrick.cpp`、`src/d4c.cpp`: 窓とリフタのcosとsinを加法定理で順次求めます。
- `src/d4c.cpp`: 非周期性を求めるとき、パワースペクトル全体をソートせず、小さいほうの値だけを選びます。

最初の2つは結果を変えません。残りの2つは丸め誤差の範囲で結果が変わります（スペクトルの相対差は3.5e-8以下）。

- 上流プロジェクト: https://github.com/mmorise/World
- 取得コミット: `d625e7608ca23a870018f01e7c562ac683d9847f`
- ライセンス: modified BSD（`LICENSE.txt`）

FFT実装の上流許諾は `OOURA-NOTICE.txt` に、`src/world/macrodefinitions.h` が含むMIT通知は `MACRODEFINITIONS-LICENSE.txt` に記載しています。

UtauTTSではこのソースを独自の解析・合成ライブラリとしてビルドします。
