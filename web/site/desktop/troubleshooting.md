---
title: 困ったとき
description: デスクトップ版で音源が出ない、起動しないときの確認点
---

# 困ったとき

## 音源が一覧に出ない

- 音源が、起動ファイルと同じ場所の`voice/音源名/`に置かれているか確かめます。
- 音源フォルダの中に`oto.ini`があるか確かめます。
- 置いたあとに、「ファイル」→「音源とプラグイン」→「音源を再読込」を選びます。

詳しい置き方は[音源の追加](/desktop/voicebanks)を参照してください。

## resamplerやwavtoolが一覧に出ない

実行ファイルを`Resamplers`または`Wavtools`フォルダへ置き、「ファイル」→「音源とプラグイン」→「Classic UTAUを再読み込み」を選びます。

## 「utautts-openjtalk-features not found」と出る

文章の解析に使うファイルが見つかっていません。ZIPを展開し直してください。セキュリティソフトが実行ファイルを削除していることがあります。除外の設定は、`utautts.exe`だけでなく展開したフォルダごとに指定してください。

Linux版では、`runtime/utautts-openjtalk-features`に実行権限があるかも確かめてください。

## 読みやモーラ数が合わないというエラーが出る

- 固有名詞や略語を[辞書設定](/desktop/settings#辞書設定)に登録します。
- 長音（ー）、促音（っ）、拗音（ゃゅょ）を含む読みを確かめます。
- それでも直らない場合は、入力した文章を添えて報告してください。

## Linuxで起動しない

端末から`./utautts`を実行し、足りないライブラリの名前を確かめます。Qt Quick、Qt Quick Controls、Qt Multimediaのパッケージが必要です（[インストール](/desktop/#linux)）。ZIPの展開で実行権限が外れた場合は、`chmod +x`を実行し直してください。

MangoHudを有効にしていると、音声の初期化で終了することがあります。その場合は、MangoHudを無効にして起動してください。

```bash
MANGOHUD=0 ./utautts
```

## Linuxで日本語が四角い記号になる

日本語のフォントが入っていません。Debian・Ubuntuでは、`sudo apt-get install fontconfig fonts-noto-cjk`を実行してから起動し直してください。

## 音声ができない・合成が遅い

合成中は、ログの画面に処理の内容が出ます。文章、音源、Renderer、抑揚モデルの組み合わせを確かめてください。初期のRendererと短い文章で試すと、原因を絞り込めます。

## 不具合を報告する

「ヘルプ」→「診断情報を書き出す...」でJSONを保存し、操作の手順と画面に出たエラーを添えて[GitHubのIssues](https://github.com/yh2237/UtauTTS/issues)へ報告してください。診断情報には、入力した文章、音声、音源の場所は記録されません。
