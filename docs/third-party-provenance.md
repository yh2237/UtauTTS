# 第三者コードの出典

UtauTTSが互換処理や移植のために参照した公開コードと、それを使うモジュールの一覧です。ライセンスの本文は[`THIRD_PARTY_NOTICES.txt`](../THIRD_PARTY_NOTICES.txt)と[`licenses/`](../licenses/)にあります。

## OpenUtauとの互換処理

次のモジュールはOpenUtau（MIT License）の実装を参照しています。

- 音素のタイミング（`internal/render/world/`）: 音素の境界の決め方と、閉鎖音・破裂音の処理
- プロジェクトの入出力（`internal/openutau/`、`cmd/tools/utautts-ustx/`）: OpenUtauのプロジェクトファイル（USTX）の書き出し
- 英語のC+Vの発音形式（`internal/frontend/english_phonemizer.go`）: Cadlaxa氏のEnglish C+V Phonemizerに基づく、エイリアスを探す順序

## WORLD

`utautts-world-phrase`は、公式のWORLD（Harvest、CheapTrick、D4C、Synthesis）のC++のソースからビルドしています。ソースは`third_party/world/`にあります。解析を速くするため、`fft.cpp`、`harvest.cpp`、`cheaptrick.cpp`、`d4c.cpp`に変更を加えています。変更の内容は`third_party/world/README.md`にあります。

## 参照照合

次のモジュールはPythonの実装をGoに移したものです。

- Kokoro-82Mの推論（`internal/kokoro/`）: kokoroのPythonパッケージ（Apache License 2.0）。その一部はStyleTTS 2（MIT License）を改変したもの
- WavLM base+の特徴量（`UtauIDX/hubert`）: Hugging Face Transformersの実装（Apache License 2.0）と出力を照合
- 英語の辞書の引き方（`internal/refspeech/lexicon.go`）: misaki（Apache License 2.0）の辞書の引き方と、前後の語で変わる読み
- 中国語の音素の書き方（`internal/refspeech/zhuyin.go`）: misaki（Apache License 2.0）の中国語G2Pが使う注音符号の表
