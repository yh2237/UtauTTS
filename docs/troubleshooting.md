# トラブルシューティング

## 音源が一覧に表示されない

- 音源が実行ファイルと同じ階層の`voice/<音源名>/`に置かれているか確認します。
- `oto.ini`が音源フォルダ内にあるか確認します。
- 音源が複数のフォルダで包まれていても、音源ルートを再帰的に探します。
- 配置後に「ファイル」→「音源を再読込」を実行します。

## resamplerやwavtoolが一覧に表示されない

実行ファイルを`Resamplers/`または`Wavtools/`へ置き、「ファイル」→「Classic UTAUツールを再読み込み」を実行します。DLLは実行ファイルと同じサブフォルダへ置けます。

## `utautts-openjtalk-features not found` と表示される

Open JTalkの解析helperが見つかっていません。配布版では実行ファイルの隣にある`runtime`ディレクトリへ次のファイルが含まれます。

```text
runtime/utautts-openjtalk-features.exe
runtime/open_jtalk_dic_utf_8-1.11/
```

Linux版では`runtime/utautts-openjtalk-features`に実行権限があることも確認してください。

ファイルがなければ配布ZIPを再展開してください。セキュリティソフトが実行ファイルを消しやがることもあります。除外設定は`utautts.exe`だけでなく展開先のフォルダへ指定します。

## `Open JTalk frontend failed` が表示される

helper、Open JTalk辞書、音源の読み込みに問題がないか確認します。入力文章の読みが特殊な場合は「設定」→「辞書設定...」で表記と読みを登録してから文章を再解析してください。

CLIやServerで個別にruntimeの場所を指定する場合は`--openjtalk-features`と`--openjtalk-dictionary`を使えます。

## モーラ数や読みが一致しないエラーが表示される

UtauTTS側の読みとOpen JTalk側の解析結果が一致していない可能性があります。

- 固有名詞や略語などを辞書へ登録します。
- 長音、促音、拗音を含む読みを確認します。
- 文章を変更して再解析し表示された読みとモーラ列を確認します。
- それでも解決しない場合はエラーログと入力文章を添えて報告してください。

## Rendererやruntime assetが見つからない

まず`waveform` Rendererで合成できるか確認してください。WORLD系Rendererに必要なファイルは[モデル／Rendererプラグイン](plugins.md)にあります。

## LinuxでGUIが起動しない

端末から`./utautts`を実行して不足しているライブラリ名を確認します。`ldd ./utautts | grep 'not found'`でも共有ライブラリを確認できます。Qt Quick、Qt Quick Controls、Qt Multimediaの実行パッケージが必要です。ZIP展開後に実行権限が失われた場合はREADMEに記載した`chmod +x`を実行してください。

### glibc 2.44以降でCOPY relocationのエラーが出る

`COPY relocation against non-copyable protected symbol`や`GNU_PROPERTY_1_NEEDED_INDIRECT_EXTERN_ACCESS`を含むエラーは、f5dcc81より前に作られたLinux GUIバイナリで起きることがあります。古いZIPを再展開しても直らないため、この修正を含む新しいリリースを使用してください。ソースからビルドする場合は`./build.sh linux`で、ビルド直後とパッケージ展開後のELF検査も実行されます。

### MangoHudを有効にすると音声初期化時に落ちる

MangoHudのVulkanレイヤーとQt MultimediaのFFmpegプラグインの組み合わせで、`vkCreateDevice`付近のクラッシュが起きることがあります。これは上のローダーエラーとは別の問題です。切り分けや回避には次を使ってください。

```bash
MANGOHUD=0 ./utautts
```

MangoHud側の互換性問題を解消するまでの回避策です。

## Linuxで日本語が四角い記号（豆腐）になる

日本語グリフを含むフォントがインストールされていません。Debian／Ubuntuでは`sudo apt-get install fontconfig fonts-noto-cjk`を実行してからGUIを起動し直してください。`fc-list :lang=ja`で利用可能な日本語フォントを確認できます。

## 音声が生成されない、または合成が遅い

合成中はログウィンドウに処理内容が表示されます。入力文章、音源、Renderer、モデル、runtimeの組み合わせを確認してください。まずは`waveform`、抑揚モデルなし、短い文章で試すと原因を切り分けやすいです。

## 問題を報告する

「ヘルプ」→「診断情報を書き出す...」からJSONを保存し、操作手順と画面に出たエラーを添えてIssueを送ってください。入力文章、音声、ボイスバンクの絶対パスは記録されません。
