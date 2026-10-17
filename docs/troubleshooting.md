# トラブルシューティング

## 音源・ツールの認識に関する問題

### 音源が一覧に表示されない
- 配布フォルダ直下の `voice/<音源名>/` に置いてあるか確認してください。
- フォルダ内に `oto.ini` があるか確認してください。`voice/音源名/音源名/oto.ini` のように、余分なフォルダに包まれた構成も読み込めます。
- 音源を追加したあとは、「ファイル」→「音源とプラグイン」→「音源を再読込」を実行してください。

### ResamplerやWavtoolが表示されない
- 実行ファイルを `Resamplers/` か `Wavtools/` に置き、「ファイル」→「音源とプラグイン」→「Classic UTAUを再読み込み」を実行してください。依存DLLは実行ファイルと同じフォルダに置けます。

---

## 起動・解析エラー

### `utautts-openjtalk-features not found` と表示される
- Open JTalkの解析ヘルパー（`runtime/utautts-openjtalk-features.exe`）が見つかりません。セキュリティソフトが誤検知して隔離していないか確認し、ZIPを展開し直してください。
- Linuxでは、このファイルに実行権限（`chmod +x`）があるか確認してください。

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
- まず、既定のRenderer（UtauTTS WORLD phrase）、抑揚モデルなし、短い文の組み合わせで動くか確認してください。
- 外部のResamplerなどを使っている場合は、引数が合っているか、子プロセスが止まっていないかを確認してください。

---

## 問題が解決しない場合
「ヘルプ」→「診断情報を書き出す...」で診断JSONを書き出し、再現手順とエラーメッセージを添えて [GitHub Issues](https://github.com/yh2237/UtauTTS/issues) で報告してください。診断JSONには、入力した文章や音声波形などの個人情報は含まれません。
