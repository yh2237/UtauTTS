---
title: 音源の追加
description: デスクトップ版でUTAU音源を追加する方法と、Classic UTAUのツールの置き方
---

# 音源の追加

「足立レイ ver3.5.0」を同梱しています。ほかのUTAU音源はZIPから追加するか、`voice`フォルダへ配置します。

## ZIPから追加

「ファイル」→「音源とプラグイン」→「音源を追加...」でZIPを選ぶと、`voice`フォルダへ展開されます。

## フォルダを置く

配布フォルダの`voice`へ、音源ごとにフォルダを分けて配置します。macOSでは`utautts.app`と同じ階層です。

```text
voice/
  音源名/
    oto.ini
    *.wav
```

`voice`フォルダは「ファイル」→「音源とプラグイン」→「音源フォルダを開く」で開けます。配置後は同じメニューの「音源を再読込」（`Ctrl+O`）を選ぶか、UtauTTSを再起動してください。

## 音源の情報を見る

「ヘルプ」→「ボイスバンクの詳細...」で、音源に付属する説明を確認できます。

## Classic UTAUのresamplerとwavtool

UTAU互換のresamplerとwavtoolでも合成できますが、あまり推奨していません。

1. resamplerの実行ファイルを`Resamplers`フォルダ、wavtoolを`Wavtools`フォルダへ置きます。必要なDLLは、実行ファイルと同じフォルダへ置いてください。
2. 「ファイル」→「音源とプラグイン」→「Classic UTAUを再読み込み」を選びます。
3. 「設定」→「設定...」のRendererタブで「Classic UTAU」を開き、使うresamplerとwavtoolを選びます。
4. カードの設定で、Rendererを「Classic UTAU」にします。

## DiffSinger音源

Windows x64のFull配布ではDiffSinger音源にも対応しています。配置と対応範囲は[DiffSinger](https://github.com/yh2237/UtauTTS/blob/main/docs/diffsinger.md)を参照してください。
