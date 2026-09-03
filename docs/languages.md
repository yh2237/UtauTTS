# 多言語ボイスバンク

UtauTTSは、文章を発音単位とUTAU alias候補へ変換する処理をPhonemizerとしてRendererから分離しています。Rendererは言語を解釈せず、Phonemizerとvoicebank resolverが選んだ原音を合成します。

## 対応状況

| 言語 | Phonemizer | 入力 | 対応する主な音源 |
|---|---|---|---|
| 日本語 | `ja-kana` | 日本語文章またはかな | CV、VCV、CVVC |
| 英語 | `en-arpasing` | ARPAbet読み、または辞書付き英単語 | ARPAsing |
| 中国語 | `zh-cvvc` | 簡体字・繁体字、または無声調Pinyin読み | Mandarin CVVC/CVV |

CLIでは`--language ja|en|zh`を指定します。`--phonemizer`を省略すると言語に合う方式が選ばれます。HTTP APIでは同じ値を`language`と`phonemizer`へ渡します。

### 英語

現段階では、`--reading`へ空白区切りのARPAbetを指定するのが確実です。強勢番号はalias生成時に除去されます。

```powershell
utautts-cli --language en --reading "HH AH0 L OW1" --voicebank voice/en --out hello.wav
```

`--text`を使う場合は、辞書JSONへ単語ごとのARPAbet読みを登録します。未登録語を推測するG2Pはまだ行わず、曖昧な発音を黙って選ばない設計です。

### 中国語

中国語文章は無声調Pinyinへ変換します。多音字を明示したい場合は、`--reading`へ空白区切りのPinyinを指定してください。声調数字はalias生成時に除去されます。

```powershell
utautts-cli --language zh --text "你好" --voicebank voice/zh --out nihao.wav
utautts-cli --language zh --reading "ni3 hao3" --voicebank voice/zh --out nihao.wav
```

現在の日本語用prosodyモデルは英語・中国語へ適用しません。両言語では固定長・固定ピッチを基準に合成し、言語別prosodyは今後追加します。

## alias生成

英語はARPAsingの音素間aliasを優先し、単独音素へフォールバックします。中国語は`- ni`、`i hao`、`hao`の順で主aliasを探し、必要なら`i h`のようなVC接続を組み合わせます。prefix.mapとsubbankの接頭辞・接尾辞は従来どおりresolverが適用します。
