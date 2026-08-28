# Patched OpenUtau worldline

`win-x64/worldline.dll`と`linux-x64/libworldline.so`は、OpenUtau 0.1.565のworldlineをUtauTTS向けにビルドしたものです。

変更内容は次の四つです。

- `tools/worldline-parallel.patch`: `PhraseSynth`の原音解析を並列に実行し、元の順序で結合するAPIを追加します。
- `tools/world-thread-local.patch`: WORLDの疑似乱数状態をスレッドごとに分離します。CheapTrickとD4Cは解析ごとに再シードするため、逐次実行と同じ結果を保てます。
- `tools/worldline-permissive.patch`: GPLのspline依存を独自の自然三次スプライン実装へ置き換え、共有ライブラリから未使用の音声出力、Classic CLI、NumPyデバッグ出力を除外します。これによりAbseilとlibnpyも配布バイナリの依存から外れます。
- `tools/worldline-native-bridge.patch`: Go製bridge用の小さなC ABIと、原音解析のLRUキャッシュを追加します。

元にしたOpenUtauのタグは`0.1.565`、コミットは`a60ca5830b9064556157245d4bf8f5920d93e5f8`です。ビルドには同タグの`cpp`ディレクトリとBazel 6.5.0を使用しました。ライセンスは`THIRD_PARTY_NOTICES.txt`を参照してください。

再ビルドするときは、OpenUtauのリポジトリルートで`worldline-parallel.patch`、`worldline-permissive.patch`、`worldline-native-bridge.patch`の順に適用し、`world-thread-local.patch`の内容を`cpp/third_party/world.patch`の末尾へ追加します。その後、`cpp`で次を実行します。

```text
# Windows x64
bazel build //worldline:worldline -c opt --cpu=x64_windows

# Linux x64
bazel build //worldline:worldline -c opt --config=linux
```

SHA-256:

- Windows x64: `655D918375643BAD1A3FF95E9E76F0B560B6CAA009370BB56498407D1F5F0C28`
- Linux x64: `EEAE80212191C84EF2A1EBCD33567F47D9700F8E74136578944DBBEEE209136C`
