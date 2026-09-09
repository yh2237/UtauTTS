# 読み上げ品質の評価

## 多言語の診断

`english-v1.json`と`chinese-v1.json`は初期診断用の固定文です。自動読みと明示読みを比較します。正解発音の網羅的な評価セットではなく、数字・多音字など未対応の可能性があるケースも含みます。

```powershell
go run ./cmd/tools/tts-eval --voicebank <english-bank> --corpus tools/evaluation/english-v1.json --diagnose --out out/english-diagnosis
go run ./cmd/tools/tts-eval --voicebank <chinese-bank> --corpus tools/evaluation/chinese-v1.json --diagnose --out out/chinese-diagnosis
```

`diagnostics.json`へ読み、frontend単位、alias候補ヒントと候補探索結果を保存します。stressとtoneはfrontend単位に含まれます。音素化・候補探索の失敗はケースごとに保存し、全ケース処理後に終了コード1を返します。音響特徴の確認には原音WAVを読みますが、Rendererや音声生成用bridgeは起動しません。

`coverage`は探索が失敗したケースでも全位置を診断します。`positions`は休止を除く位置数、`covered`は有効な主候補がある位置数です。`candidate_counts`は休止も含むfrontend単位と同じindexで、通常の候補絞り込み後の数を保存します。`missing`には不足位置、探索したalias、使用不能なWAVなどの除外理由を残します。促音の無音closureも有効候補に含むため、この数値は実録音の充足率や発音の正しさを示しません。語尾・transitionの完全性も別途確認が必要です。全文探索が失敗した場合、経路選択結果の`lattice`は保存されません。

各ケースの`language`、`phonemizer`、`reading`を指定できます。省略時は既存の日本語既定値、または指定言語の既定phonemizerを使います。Delta/VCCV音源ではケースの`phonemizer`を`en-delta`／`en-vccv`へ変更してください。音声も作る場合は`--diagnose`を外し、例えば`--renderers waveform --model none`を指定します。出力先は常に新規ディレクトリが必要です。

## 日本語・CUDA比較

`--model-file <JSON>`で未同梱のモデルを直接比較できます。`--model none`は学習済み抑揚なしです。各WAVとともに原音選択・予定時刻を含むPlan JSONを保存します。Planは音声から実測した音素境界ではありません。

`go run ./cmd/tools/tts-eval --voicebank <voicebank> --out out/japanese-baseline` で、`tools/evaluation/japanese-v1.json` の8文を生成します。出力先は新規ディレクトリを指定してください。WAV・TXT・LAB、`report.json`、WORLDブリッジの任意プロファイルを保存します。

CPU/CUDA比較は `--renderers utautts-world-phrase,utautts-world-phrase-cuda` とし、同じプロセスで2回以上実行してください。解析キャッシュを共有するため、各レンダラーの初回とウォーム実行を分けて見ます。RTFは合成時間÷音声時間で、保存時間は含みません。ピーク、RMS、無音ユニット数は診断値であり、自然さや発音抜けの代替ではありません。語尾、疑問文から平叙文への切替、促音、長音、無声化、数字、長文の息継ぎは聴取で確認します。

2026-09-05の足立レイver3.5.0・8文×2回では、先頭1件を除く特徴量混合の平均がCPU約2.35 ms、CUDA約10.83 ms、波形生成がCPU約325 ms、CUDA約322 msでした。CUDA化は特徴量混合だけで、波形生成はCPUです。このため現在は正式版扱いにせず、波形生成の高速化、長文でのメモリ安定性、キャンセル、CPUとの数値一致、聴取評価を追加検証します。

日本語向け配布は次で作成します。

```powershell
.\tools\build-release.ps1 -Profile Japanese -OutputDirectory 'D:\project\UtauTTS\build\japanese-release'
```

このプロファイルはDiffSingerブリッジとWORLDLINE DLLを省き、独自WORLDエンジンと日本語解析を残します。OpenJTalk内包ヘルパーやQtは残るため、外部依存が全てなくなるわけではありません。省略したレンダラーを使うには別途ランタイムが必要です。GUI・サーバーZIPの展開先スモークテストは完了しており、省略対象のファイルサイズは合計約101.5 MiBです。
