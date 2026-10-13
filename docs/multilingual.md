# 日本語・英語・中国語の読み上げ

音源に合う言語と発音形式を選んでください。日本語・英語・中国語に対応しています。

## 言語と発音形式を選ぶ

| 言語 | `language` | `phonemizer` | 入力 |
| --- | --- | --- | --- |
| 日本語 | `ja` | `ja-kana`（既定） | 日本語文章またはかな読み |
| 英語 | `en` | `en-arpasing`（既定） | 英文またはARPAbet読み |
| 英語 | `en` | `en-delta` | 英文またはARPAbet読み |
| 英語 | `en` | `en-vccv` | 英文またはARPAbet読み |
| 英語 | `en` | `en-cv` | 英文またはARPAbet読み（C+V音源） |
| 中国語 | `zh` | `zh-cvvc`（既定） | 中国語文章または声調付きPinyin |

GUIでは音源と言語を選び、発音形式は通常`自動`を使います。固定する場合はプルダウンで形式を選びます。DiffSinger音源は[DiffSinger](diffsinger.md)を参照してください。

CLIでは次のように指定します。音源パスとphonemizerは使用する音源に合わせて変更してください。

```powershell
utautts-cli --voicebank "./voice/english-bank" --language en --phonemizer en-arpasing --text "Hello world." --renderer utautts-world-phrase --out hello.wav
```

HTTP APIでは`language`と`phonemizer`を指定します。詳細は[Serverの合成API](server.md)を参照してください。

## 読みを指定する

自動読みが誤る場合は読みを直接指定するか[辞書設定](settings.md#辞書設定)で修正します。CLIでは`--text`の代わりに`--reading`を指定できます。

| 言語 | 読みの例 | 区切り |
| --- | --- | --- |
| 英語 | `HH AH0 L OW1 \| W ER1 L D \| SP` | 空白は音素区切り。`\|`は単語境界。`SP`は休止 |
| 中国語 | `ni3 hao3 \|` | 空白は音節区切り。`\|`は休止 |

英語の母音末尾の数字は強勢を表し、`0`は無強勢、`1`は第一強勢です。中国語の数字は声調を表します。テキスト入力では句読点を休止へ変換します。中国語の空白だけでは休止を入れません。

## 自動読みと韻律

英語はユーザー辞書・音源辞書・同梱CMUdictの順で読みを使い、未登録語は語幹や綴りから推定します。固有名詞・略語・不規則な綴りは、読みを指定するか辞書で修正してください。辞書の強勢は保持しますが、綴りからの推定では強勢は未確定です。

数字・小数・桁区切り・時刻・百分率の読みを展開します。日付や通貨の読みと同綴異音語の文脈判定は網羅していません。

英語の抑揚を使うには、ピッチ処理を有効にし、抑揚の強さを0より大きくします。長さは強勢・子音・句末に応じて調整しますが、文の意味に応じた強調は扱いません。

Delta・VCCV音源では、無強勢の`AH0`に弱母音の原音を優先します。対応する原音がない場合は通常の`AH`を候補に残します。強勢が未指定の`AH`を自動で弱母音には変えません。語末と次の語頭をまとめた接続音がない場合は、語末子音群の録音も探します。

C+V音源はARPAbetの各音素に1つの原音を使います。原音名の優先順はOpenUtauのEnglish C+V Phonemizer（Cadlaxa）に準拠します。

| 位置 | 母音 | 子音 |
| --- | --- | --- |
| 文頭 | `-V`→`- V`→`V` | `- C`→`-C`→`C` |
| 文中 | `-V`→`V`→`- V` | `C`→`-C`→`- C` |
| 語末 | `V -`／`V-` | `C -` |

`-V`・`-C`は空白なしの表記です。

英語の機能語は、句の途中で前後の語が休止でない場合に弱形で読みます。たとえば句中の`of`は無強勢の`AH0 V`になります。単独・句の先頭・末尾では入力どおりに読み、強く読みたい場合は`AH1 V`のように読みを指定してください。ユーザー辞書と音源辞書の明示指定は弱形化より優先します。

中国語は数値・小数を読み上げ、年の数字は一桁ずつ読みます。「一」「不」・第三声の変調と軽声を調整します。多音字は内蔵辞書で読み分け、未登録語は文字単位で推定します。複雑な文脈や第三声の連続では、正しく読めない場合があります。

中国語の`ü`は読みで`v`または`u:`と書けます。`nüe/lüe`の原音名には`nve/lve`と`nue/lue`の両表記を探し、`presamp.ini`の母音・子音の分類も、この表記の違いを区別せずに扱います。`nu/lu`と`nv/lv`は別の母音として扱います。

英語の既定抑揚モデルはEnglish Frame Intonation TCN v1、中国語はMandarin Tone Intonation v1です。中国語モデルは声調規則の音高曲線を補正します。

## 音が欠ける場合

UtauTTSで生成できるのは、基本的に音源へ収録されている子音と接続音です。音が欠ける場合は、音源と発音形式の組み合わせを確認してください。[読み上げ品質の評価](../tools/evaluation/README.md)では、原音候補と必須音の不足を調べられます。

本体とWORLDブリッジは同じバージョンを使用してください。

[選択した原音の確認](../tools/evaluation/README.md#選択した原音を確認する)では切り出し範囲と合成結果を比較できます。

音素表記の参考は[OpenUtauのVCCV対応表](https://github.com/stakira/OpenUtau/blob/master/OpenUtau.Plugin.Builtin/EnglishVCCVPhonemizer.cs)、[OpenUtauのC+V対応表](https://github.com/stakira/OpenUtau/blob/master/OpenUtau.Plugin.Builtin/EnglishCpVPhonemizer.cs)と[香港理工大学のPinyin綴り規則](https://www.polyu.edu.hk/bepth/introduction-to-phonetics/spelling-rules-in-pinyin/?sc_lang=en)を参照してください。

## 原音区間の自動適用

CPU版WORLDの英語・中国語は登録済みの原音区間を自動で使います。通常は事前解析や追加モデルの取得は不要です。適用条件と区間の登録方法は[原音区間ライブラリ](source-understanding.md)を参照してください。

同梱データの対象は[原音区間ライブラリの収録範囲](../models/source-phones/README.md)を参照してください。別のボイスバンクに区間を登録する手順は[原音区間ライブラリ](source-understanding.md)にあります。

## 長さと合成計画

英語・中国語は子音・母音・韻尾の長さを合計して発話時間を決めます。CLIの`--mora-ms`は音素時間配分の基準値です。手動で指定した各単位の長さを優先します。

`--plan-out`で合成計画を保存し、音素時刻と原音の対応を確認できます（[診断項目](../tools/evaluation/README.md)）。学習手順は[モデルの学習](model-training.md)を参照してください。
