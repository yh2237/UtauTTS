# 原音の音素区間ライブラリ

CPU版WORLDの英語・中国語合成で使用する原音区間を記録しています。原音のハッシュと音素列が一致する場合に自動適用し、未登録の原音や対応が曖昧な区間はotoに基づく処理を使います。

## 収録範囲

| ファイル | 音源 | 対象 |
| --- | --- | --- |
| `en.json` | 重音テト英語音源、暗鳴ニュイ EnglishVCCV -DIVA- | 一部の語末子音原音 |
| `zh.json` | Kaze_Imo_CVVCHN | `jin`・`hen` |
| `zh.json` | Artes_-JinZhan-_CVVCHS | `hen` |

中国語は単母音と鼻音韻尾を含む音節全体を対応付けます。複合母音は対象外です。

## 音源独自の区間を登録する

ボイスバンク直下に`source-phone-library.json`を置くと、同梱ライブラリより優先して使用します。作成手順・データ形式・自動適用の設定は[原音区間ライブラリ](../../docs/source-understanding.md)を参照してください。

## 出典

音素区間の作成にはMontreal Forced Aligner（MFA）の音響モデルを使用しています。出典と利用条件は各通知を参照してください。

- 英語: [MFA English ARPA通知](../../licenses/MFA-English-ARPA-NOTICE.txt)
- 中国語: [MFA Mandarin通知](../../licenses/MFA-MANDARIN-NOTICE.txt)

原音の録音データと整列モデルの重みはこのライブラリに含みません。
