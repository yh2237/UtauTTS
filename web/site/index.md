---
title: UtauTTS
description: UtauTTSのデスクトップ版とWeb版の使い方
---

# UtauTTS

UtauTTSは、UTAU音源の声で日本語・英語・中国語の文章を読み上げるTTSです。文章を入れると読みと抑揚を自動で付け、モーラごとの高さや長さを手で直せます。

パソコンに入れて使うデスクトップ版と、ブラウザで開くだけで使えるWeb版があります。プロジェクト（`.utautts`）は共通なので、途中からもう一方で続けられます。

## デスクトップ版とWeb版の違い

| | デスクトップ版 | Web版 |
| --- | --- | --- |
| 始め方 | ZIPを展開して起動 | [エディタ](/editor/){target="_self"}を開く |
| 対応環境 | Windows（x64）、macOS（Apple Silicon）、Linux（x64） | パソコンとスマートフォンのブラウザ |
| 音源 | `voice`フォルダに置いた音源をいつでも使える | ZIPで追加し、ページを開いている間だけ使える |
| 合成方式 | UtauTTS WORLD phrase、Classic UTAU（resamplerとwavtool）、DiffSinger | UtauTTS WORLD phrase |
| 書き出し | WAV、USTX、AviUtl向けのexo | WAV、USTX |
| そのほか | コマンドライン版とHTTP Server版もある | |

## 使い方

- [デスクトップ版の使い方](/desktop/): インストールから書き出し、設定まで
- [Web版の使い方](/web/): エディタの開き方から保存、スマートフォンでの操作まで
- [利用条件とライセンス](/terms): 作った音声の利用条件と、UtauTTSのライセンス
