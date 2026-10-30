# トラブルシューティング

## 音源・ツールの認識に関する問題

### 音源が一覧に表示されない
- 配布フォルダ直下の `voice/<音源名>/` に置いてあるか確認してください。
- フォルダ内に `oto.ini` があるか確認してください。`voice/音源名/音源名/oto.ini` のように、余分なフォルダに包まれた構成も読み込めます。
- 音源を追加したあとは、「ファイル」→「音源」→「音源を再読込」を実行してください。

---

## 起動・解析エラー

### 起動しない（ウィンドウが出ない）
- 実行ファイルと同じフォルダの `startup-error.log` に、理由とQtのバージョンが残ります。端末から `./utautts` を実行した場合も同じ内容が表示されます。
- 起動できるかだけを確かめるには `./utautts --startup-check` を使います。音声と合成は行いません。
- 報告するときは、`startup-error.log` か端末の出力を添えてください。終了コードが 2 のときは更新中のロックが残っています。表示された `*.update-lock.json` を削除すると起動できます。

### `utautts-openjtalk-engine.dll not found` と表示される
- Open JTalkのライブラリ（Windowsは`runtime/utautts-openjtalk-engine.dll`、Linuxは`.so`、macOSは`.dylib`）が見つかりません。セキュリティソフトが誤検知して隔離していないか確認し、ZIPを展開し直してください。

### `Open JTalk frontend failed` が表示される
- 特殊な記号や固有名詞の読みを解析できなかった可能性があります。「設定」→「辞書設定...」でその語の読みを登録してください。

### Linux環境で文字が四角（豆腐）になる
- 日本語フォントが足りません。次のコマンドで入れてください。
  ```bash
  sudo apt-get install -y fonts-noto-cjk fontconfig
  ```

### Linuxで `COPY relocation against non-copyable protected symbol` エラーが出る
- glibc 2.44以降で起きることがあります。最新リリースのバイナリを使うか、最新のソースコードから `./build.sh linux` でビルドし直してください。

---

## 合成・再生の問題

### 音声が生成されない / 合成が極端に遅い
- まず、抑揚モデルなし、短い文の組み合わせで動くか確認してください。

---

## 問題が解決しない場合
再現手順とエラーメッセージを添えて [GitHub Issues](https://github.com/yh2237/UtauTTS/issues) で報告してください。
