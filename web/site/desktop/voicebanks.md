---
title: 音源の追加
description: デスクトップ版でUTAU音源を追加する方法と、Classic UTAUのツールの置き方
---

# 音源の追加

最初から「足立レイ ver3.5.0」が入っています。手持ちのUTAU音源は、次のどちらかの方法で追加します。

## ZIPから追加する

配布されたZIPのままなら、「ファイル」→「音源とプラグイン」→「音源を追加...」でZIPを選びます。ZIPを`voice`フォルダへ展開して読み込みます。同じ名前の音源フォルダがある場合は置き換えます。

## フォルダを置く

起動ファイルと同じ場所にある`voice`フォルダへ、音源ごとにフォルダを分けて置きます。

```text
voice/
  音源名/
    oto.ini
    *.wav
```

`voice`フォルダは「ファイル」→「音源とプラグイン」→「音源フォルダを開く」で開けます。置いたあとは、同じメニューの「音源を再読込」（`Ctrl+O`）を選ぶか、UtauTTSを起動し直してください。

音源フォルダがもう1階層のフォルダに入っていても読み込めます。それより深い場合は、音源フォルダを`voice`の直下へ移してください。

## 音源の情報を見る

「ヘルプ」→「ボイスバンクの詳細...」で、読み込んだ音源の画像、原音の数、音源に入っている説明を見られます。音源の利用条件は、それぞれの音源の説明や配布元の規約で確かめてください。

## Classic UTAUのresamplerとwavtool

UTAU互換のresamplerとwavtoolでも合成できます。

1. resamplerの実行ファイルを`Resamplers`フォルダ、wavtoolを`Wavtools`フォルダへ置きます。必要なDLLは、実行ファイルと同じフォルダへ置けます。
2. 「ファイル」→「音源とプラグイン」→「Classic UTAUを再読み込み」を選びます。
3. 「設定」→「設定...」の「音声合成」にあるRendererのタブで「Classic UTAU」を開き、使うresamplerとwavtoolを選びます。
4. カードの設定で、Rendererを「Classic UTAU」にします。

外部のツールは、信頼できる配布元のものだけを使ってください。

## DiffSinger音源

DiffSinger音源にも対応しています。置き方と対応している範囲は[DiffSinger](https://github.com/yh2237/UtauTTS/blob/main/docs/diffsinger.md)を参照してください。
