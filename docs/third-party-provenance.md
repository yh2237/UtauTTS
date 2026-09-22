# 第三者コードの出典

UtauTTSの互換処理で参照した公開実装と対応箇所を記載します。配布物の
ライセンス本文: [`THIRD_PARTY_NOTICES.txt`](../THIRD_PARTY_NOTICES.txt)、
[`licenses/`](../licenses/)。

## OpenUtau互換処理

参照実装と対応箇所:

- `internal/render/external_utau.go` はClassic UTAUのresamplerとwavtoolへ渡す
  引数と包絡線処理を実装しています
- `internal/render/worldline*.go` は音素単位のタイミングと停止音の扱いを実装しています
- `internal/openutau/` と `cmd/tools/utautts-ustx/` はUSTX形式の入出力を実装しています
- `internal/frontend/phonemizer.go` のC+V phonemizer (`ParseEnglishCV`) は、英語C+V音源で
  試すaliasの優先順序を実装しています

参照元は[OpenUtauのworldline実装](https://github.com/openutau/OpenUtau/tree/0.1.565/cpp/worldline)、
[English VCCV phonemizer](https://github.com/stakira/OpenUtau/blob/master/OpenUtau.Plugin.Builtin/EnglishVCCVPhonemizer.cs)、
[English C+V phonemizer](https://github.com/stakira/OpenUtau/blob/master/OpenUtau.Plugin.Builtin/EnglishCpVPhonemizer.cs)（作者: Cadlaxa）です。
OpenUtauのMIT Licenseと著作権表示は`THIRD_PARTY_NOTICES.txt`に記載しています。

`utautts-world-phrase`のビルド元: リポジトリ内の公式WORLDソース。

## プロジェクト作成のアセット

画像や評価用JSONの作成元: UtauTTS。再配布条件: リポジトリのMIT License。
