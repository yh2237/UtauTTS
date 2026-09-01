# 手動調整から抑揚モデルを作る

UtauTTSのGUIでv8の自動イントネーションを調整し、その操作を教師データにして小さな補正モデルを学習できます。生成されるversion 11モデルは`frame-intonation-v8`とモーラ単位の補正headを一つのJSONへ収めたものです。音声やボイスバンクそのものは学習しません。

この機能は実験的です。少量の教師データから生成できることとv8より自然になることは同じではありません。必ず学習に使っていない文章でv8と比較してください。

## 必要なもの

- UtauTTSのソースツリー、または`tools/`と`models/frame-intonation-v8.json`を含む開発用配布物
- Python 3.10以降
- Python packageのNumPyとPyTorch
- 教師データの試聴に使うボイスバンクとRenderer

PowerShellでUtauTTSのルートディレクトリを開き、Python packageを準備します。

```powershell
python -m venv .venv
.\.venv\Scripts\Activate.ps1
python -m pip install --upgrade pip
python -m pip install numpy torch
```

学習にGPUは必要ありません。

## 1. 教師データを収集する

1. GUIの「設定」→「設定...」で「開発者モード」を有効にします。
2. 「設定」→「抑揚の教師データ生成」を開きます。
3. 文章セット、ボイスバンク、Renderer、音高、音源タイプを選んで「新しく開始」を押します。
4. 表示された文章を合成して聴き必要な点だけピッチグラフで調整します。
5. 結果を採用するなら「OK・次へ」を押します。変更しなかった文章も確認画面で明示的に採用すると0補正の教師になります。
6. 判断できない文章は「スキップ」を使用します。
7. 文章セットが終わったら「教師データを書き出す」を押します。

一度も合成していない文章は採用できません。収集中の状態は自動保存されて同じv8と辞書を利用できるなら「途中から再開」で続けられます。

文章セットは「基本確認セット（10文）」とJSUT BASIC5000から選んだ50文セットが6個あります。最初は10文で操作を確認し、その後にBASIC5000セットを集めるのがいいと思います。複数回に分けて書き出したJSONLもまとめて学習できます。

BASIC5000文章はJSUTの音声ではなくテキストだけを使っています。元テキストには田中コーパス（CC BY 2.0）、Wikipedia（CC BY-SA 3.0）、JSUT独自文（CC BY-SA 4.0）が含まれます。出典と条件は[`licenses/JSUT-DATA-AND-LABELS.txt`](../licenses/JSUT-DATA-AND-LABELS.txt)に記載しています。教師JSONLを再配布する場合もこの出典情報を一緒に示してください。

書き出しでは次の2ファイルが生成されます。

- `*.jsonl`: 採用した文章、読み、v8予測値、手動補正、編集mask、言語特徴、合成条件
- `*-report.json`: session、採用数、skip数、使用したv8などの概要

`.utautts`プロジェクトは教師データとして取り込みません。学習対象になるのは専用画面で確認して採用したJSONLだけです。

## 2. データを監査する

学習前に配列長、v8のidentity、非数値、pauseの誤編集などを検査します。

```powershell
python tools/audit-manual-intonation-data.py `
  .\data\manual-prosody-session1.jsonl `
  .\data\manual-prosody-session2.jsonl `
  --out .\out\prosody\manual-prosody-audit.json
```

エラーがあれば終了コード2になって学習へ進みません。reportでは特に次を確認してください。

- `records`: 採用した発話数
- `morae.edited_ratio`: 手動で変更した点の割合
- `offset_cents`: 補正方向と大きさ
- `model_hashes`: 収集に使ったv8が統一されているか
- `prompt_packs`: 各文章セットから採用した発話数
- `warnings`: 少量データ、補正方向の偏り、±120 cent超など

JSONLには入力文章、読み、ボイスバンクIDなどが入ります。共有や公開の前に内容と各音源の利用条件を確認してください。

## 3. モデルを学習する

次の例では複数sessionをまとめて3つのseedからvalidation成績が最もよいモデルを選びます。

```powershell
python tools/train-manual-intonation-residual.py `
  --dataset .\data\manual-prosody-session1.jsonl .\data\manual-prosody-session2.jsonl `
  --base-model .\models\frame-intonation-v8.json `
  --out .\out\prosody\my-manual-prosody-v1.json `
  --report .\out\prosody\my-manual-prosody-v1-training-report.json `
  --model-id my-manual-prosody-v1 `
  --display-name "My manual prosody v1" `
  --description "自分で確認した抑揚調整から学習" `
  --epochs 240 `
  --hidden 16 `
  --seeds 23,29,41
```

`--model-id`はGUI、CLI、Serverで使う一意なIDです。同じIDのモデルを複数置くと起動時にエラーになります。

複数JSONLに同じ文章と読みがある場合は同じgroupとして扱って引数で後に指定したsessionの採用結果を使います。異なるv8、frontend、辞書fingerprintのデータは混在させられません。

同じ文章の再編集がtrain、validation、testへ分かれないよう文章単位で固定分割します。生成JSONにはv8の重みも入るので実行時に別のv8 JSONは必要ありません。

## 4. 学習reportを判断する

reportの`metrics`には各splitについて`zero`と`model`があります。

- `zero`: v8をそのまま使い、補正を一切予測しないbaseline
- `model`: 学習した補正head
- `edited_mae_cents`: 実際に編集した点だけの平均絶対誤差
- `weighted_mae_cents`: 編集点と確認済み未編集点を教師weight込みで評価した誤差
- `portable_max_abs_error_cents`: 学習時推論とexport形式の推論差

まずvalidationの`weighted_mae_cents`がzero baselineを下回るか確認します。編集点だけ改善して未編集点を必要もなく動かすモデルではポン出し品質が悪化するかもしれません。testは学習設定を選び終えたあとの最終確認にだけ使います。

データ量の目安は次のとおりです。

- 約10発話: schema、収集操作、学習経路の動作確認
- 約30発話: 補正分布と単純baselineの確認
- 約100発話: 個人補正モデルの初回評価
- 200～300発話: 未学習文章の聴感比較と採用判断

発話数だけでなく疑問文、複数のアクセント句、長文、促音、長音、鼻音、異なるアクセント核位置を入れます。BASIC5000セットはこの偏りを減らすために選んでいますが疑問文が少ないので基本確認セットも併用してください。学習済み文章を再生して自然でも汎化性能の証明にはなりません。

## 5. UtauTTSへ追加する

評価したモデルJSONを`models/`へコピーします。

```powershell
Copy-Item `
  -LiteralPath .\out\prosody\my-manual-prosody-v1.json `
  -Destination .\models\my-manual-prosody-v1.json
```

GUIを再起動すると「抑揚モデル」の一覧へ表示されます。モデル一覧は起動時に読み込まれるので実行中にコピーした場合は再起動が必要です。

CLIでは`models/`へコピーせず検索directoryを追加して試すこともできます。

```powershell
.\utautts-cli.exe `
  --model-dir .\out\prosody `
  --prosody my-manual-prosody-v1 `
  --prosody-pitch-only `
  --apply-pitch `
  --renderer utautts-world-phrase `
  --voicebank .\voice\my-voicebank `
  --text "こんにちは、今日はいい天気です。" `
  --out .\out\my-manual-prosody.wav
```

最初はv8と作成モデルを同じ文章、ボイスバンク、Rendererで比較してください。不要ならモデルJSONを`models/`から外します。

## 再学習時の注意

- 元のJSONLは編集せず、sessionごとに保存します。
- dataset、base model、引数、seed、reportを一緒に保管します。
- v8が更新された場合は古いv8で収集したデータをそのまま新しいv8の補正学習へ使いません。
- 同じ文章を再調整した場合は新しいsessionを`--dataset`の後ろへ指定します。
