# 中国語抑揚モデルの学習

`tone-intonation-zh-v1`は、AISHELL-3の音声とPaddleSpeechの音節時刻から、声調規則の音高曲線に加える補正を学習したモデルです。学習データは同梱しません。出典と配布条件は[抑揚モデルのライセンス](../models/README.md)を参照してください。

[AISHELL-3のParquetシャード](https://huggingface.co/datasets/MatrixStudio/AISHELL-3/tree/main/data)を`data/aishell3/train-00000-of-00045.parquet`へ置き、[PaddleSpeechの音節時刻アーカイブ](https://github.com/PaddlePaddle/PaddleSpeech/tree/develop/examples/aishell3/tts3)を展開して`data/aishell3/aishell3_alignment_tone/`へ置きます。現在のモデルは1シャード内の3話者から各250発話を使用しました。AISHELL-3全体は必要ありません。

Python環境にNumPyとPyArrowを用意し、[WORLDエンジン](frame-intonation-training.md#準備)をビルドしたうえで、リポジトリのルートから実行します。

```powershell
python tools/train-mandarin-intonation.py
```

既定の出力先は`out/tone-intonation-zh-v1/tone-intonation-zh-v1.json`です。入力や出力の場所を変える場合は`--parquet`、`--alignments`、`--world-engine`、`--out`を指定します。学習器は2話者で重みを学習し、残る1話者で補正量を選びます。記録される評価値は、補正量の選択に使った話者の値であり、独立したテスト結果ではありません。
