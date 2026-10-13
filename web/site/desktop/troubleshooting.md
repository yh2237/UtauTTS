---
title: トラブルシューティング
description: デスクトップ版のトラブルシューティング
---

# トラブルシューティング

## 音源が一覧に出ない

- 配布フォルダの`voice/音源名/`に音源フォルダを置いてください。macOSでは`utautts.app`の外です。
- 音源フォルダに`oto.ini`があるか確認してください。
- 配置後に「ファイル」→「音源とプラグイン」→「音源を再読込」を実行してください。

詳しい置き方は[音源の追加](/desktop/voicebanks)を参照してください。

## resamplerやwavtoolが一覧に出ない

実行ファイルを`Resamplers`または`Wavtools`フォルダへ置き、「ファイル」→「音源とプラグイン」→「Classic UTAUを再読み込み」を選びます。

## Open JTalkの解析ファイルが見つからない

`utautts-openjtalk-features not found`は、文章解析に使うファイルが見つからないエラーです。配布ZIPを展開し直してください。セキュリティソフトがファイルを削除している場合は、展開先のフォルダごと除外設定を指定してください。

Linux版では`runtime/utautts-openjtalk-features`の実行権限も確認してください。

## 読みやモーラ数が合わない

- 固有名詞や略語は[辞書設定](/desktop/settings#辞書設定)に登録してください。
- 長音（ー）、促音（っ）、拗音（ゃゅょ）を含む読みを確認してください。

解決しない場合は、入力した文章を添えて[不具合を報告](#不具合を報告する)してください。

## Linuxで起動しない

端末から`./utautts`を実行し、不足するライブラリ名を確認してください。必要なQtパッケージは[インストール](/desktop/#linux)にあります。

MangoHudを有効にしていると、音声の初期化で終了することがあります。その場合は、MangoHudを無効にして起動してください。

```bash
MANGOHUD=0 ./utautts
```

## Linuxで日本語が四角い記号になる

日本語フォントが入っていません。Debian・Ubuntuでは`sudo apt-get install fontconfig fonts-noto-cjk`を実行し、アプリを再起動してください。

## 不具合を報告する

[GitHub Issues](https://github.com/yh2237/UtauTTS/issues)または[@2237yh](https://x.com/2237yh)へ報告してください。
