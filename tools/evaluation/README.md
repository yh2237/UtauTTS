# 日本語品質・CUDAの評価

`go run ./cmd/tools/tts-eval --voicebank <voicebank> --out out/japanese-baseline` で、`tools/evaluation/japanese-v1.json` の8文を生成します。出力先は新規ディレクトリを指定してください。WAV・TXT・LAB、`report.json`、WORLDブリッジの任意プロファイルを保存します。

CPU/CUDA比較は `--renderers utautts-world-phrase,utautts-world-phrase-cuda` とし、同じプロセスで2回以上実行してください。解析キャッシュを共有するため、各レンダラーの初回とウォーム実行を分けて見ます。RTFは合成時間÷音声時間で、保存時間は含みません。ピーク、RMS、無音ユニット数は診断値であり、自然さや発音抜けの代替ではありません。語尾、疑問文から平叙文への切替、促音、長音、無声化、数字、長文の息継ぎは聴取で確認します。

2026-09-05の足立レイver3.5.0・8文×2回では、先頭1件を除く特徴量混合の平均がCPU約2.35 ms、CUDA約10.83 ms、波形生成がCPU約325 ms、CUDA約322 msでした。CUDA化は特徴量混合だけで、波形生成はCPUです。このため現在は正式版扱いにせず、波形生成の高速化、長文でのメモリ安定性、キャンセル、CPUとの数値一致、聴取評価を追加検証します。

日本語向け配布は次で作成します。

```powershell
.\tools\build-release.ps1 -Profile Japanese -OutputDirectory 'D:\project\UtauTTS\build\japanese-release'
```

このプロファイルはDiffSingerブリッジとWORLDLINE DLLを省き、独自WORLDエンジンと日本語解析を残します。OpenJTalk内包ヘルパーやQtは残るため、外部依存が全てなくなるわけではありません。省略したレンダラーを使うには別途ランタイムが必要です。GUI・サーバーZIPの展開先スモークテストは完了しており、省略対象のファイルサイズは合計約101.5 MiBです。
