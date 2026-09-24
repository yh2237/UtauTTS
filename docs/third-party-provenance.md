# 第三者コードの出典

UtauTTSの互換処理で参照した公開実装と対応箇所を記載します。配布物のライセンス本文は[`THIRD_PARTY_NOTICES.txt`](../THIRD_PARTY_NOTICES.txt)と[`licenses/`](../licenses/)にあります。

## OpenUtau互換処理

参照実装と対応箇所は次の通りです。

- `internal/render/external_utau.go`: Classic UTAUのresamplerとwavtoolへ渡す引数と包絡線処理
- `internal/render/worldline/`: 音素単位のタイミングと停止音の扱い
- `internal/openutau/`と`cmd/tools/utautts-ustx/`: USTX形式の入出力
- `internal/frontend/phonemizer.go`のC+V phonemizer（`ParseEnglishCV`）: 英語C+V音源で試すaliasの優先順序

参照元は[OpenUtauのworldline実装](https://github.com/openutau/OpenUtau/tree/0.1.565/cpp/worldline)、[English VCCV phonemizer](https://github.com/stakira/OpenUtau/blob/master/OpenUtau.Plugin.Builtin/EnglishVCCVPhonemizer.cs)、[English C+V phonemizer](https://github.com/stakira/OpenUtau/blob/master/OpenUtau.Plugin.Builtin/EnglishCpVPhonemizer.cs)（作者: Cadlaxa）です。OpenUtauのMIT Licenseと著作権表示は`THIRD_PARTY_NOTICES.txt`に記載しています。

`utautts-world-phrase`は、リポジトリ内の公式WORLDソースからビルドしています。

## プロジェクト作成のアセット

画像や評価用JSONはUtauTTSが作成したもので、リポジトリのMIT Licenseで再配布できます。
