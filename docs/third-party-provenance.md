# 第三者コードの出典

UtauTTSが互換処理や移植のために参照した公開コードと、該当モジュールの一覧です。ライセンス本文は [`THIRD_PARTY_NOTICES.txt`](../THIRD_PARTY_NOTICES.txt) と [`licenses/`](../licenses/) にあります。

## OpenUtauとの互換処理

次のモジュールはOpenUtau（MIT License）の実装を参照しています。

- 音素のタイミング（`internal/render/world/`）: 音素境界の決め方と閉鎖音・破裂音の処理
- プロジェクトの入出力（`internal/openutau/`、`cmd/tools/utautts-ustx/`）: OpenUtauのプロジェクトファイル（USTX）の書き出し
- 英語C+Vの発音形式（`internal/frontend/english_phonemizer.go`）: Cadlaxa氏のEnglish C+V Phonemizerに基づくエイリアスの探索順

## WORLD

`utautts-world-phrase` は、公式WORLD（Harvest、CheapTrick、D4C、Synthesis）のC++ソース（`third_party/world/`）からビルドしています。解析を高速化するため `fft.cpp`、`harvest.cpp`、`cheaptrick.cpp`、`d4c.cpp` に変更を加えており、変更内容は [`third_party/world/README.md`](../third_party/world/README.md) にまとめています。

## 参照照合

次のモジュールはPython実装をGoへ移植、または出力照合に参照したものです。

- Kokoro-82Mの推論（`internal/kokoro/`）: kokoroのPythonパッケージ（Apache License 2.0）。一部はStyleTTS 2（MIT License）の改変
- WavLM base+の特徴量（`UtauIDX/hubert`）: Hugging Face Transformersの実装（Apache License 2.0）と出力を照合
- 英語の辞書参照（`internal/refspeech/lexicon.go`）: misaki（Apache License 2.0）の辞書引きと、前後の語に応じた読みの切り替え
- 中国語の音素変換（`internal/refspeech/zhuyin.go`）: misaki（Apache License 2.0）の中国語G2Pが使う注音符号の対応表
