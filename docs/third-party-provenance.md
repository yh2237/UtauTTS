# 第三者コードの出典

UtauTTSの互換処理において参照したオープンソース実装と該当モジュールの一覧です。ライセンス本文は [`THIRD_PARTY_NOTICES.txt`](../THIRD_PARTY_NOTICES.txt) および [`licenses/`](../licenses/) を参照してください。

## OpenUtau 互換実装

- **Classic UTAU連携** (`internal/render/external_utau.go`):
  resamplerおよびwavtoolへの引数受け渡し規則、および5点包絡線処理。
- **音素タイミング** (`internal/render/worldline/`):
  音素境界の決定および閉鎖音・破裂音の処理。
- **プロジェクト入出力** (`internal/openutau/`, `cmd/tools/utautts-ustx/`):
  OpenUtauのプロジェクトファイル形式（USTX）の読み書き。
- **英語 C+V Phonemizer** (`internal/frontend/english_phonemizer.go`):
  Cadlaxa氏によるEnglish C+V Phonemizerに基づくエイリアス探索優先順。

※上記はOpenUtau（MIT License）の実装を参照しています。

## WORLD 音声分析合成系

`utautts-world-phrase` は、公式のWORLDライブラリ（Harvest, CheapTrick, D4C, Synthesis）のC/C++ソースコードからビルドされています。
