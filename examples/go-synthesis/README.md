# 合成APIの開発用動作確認

## 実行

Go 1.27以上と、UtauTTSの音源・モデル・Renderer・runtimeが必要です。

```sh
go mod tidy
go run . -assets /path/to/UtauTTS -out speech.wav
```

`-assets`には次の構成のディレクトリを指定します。

```text
UtauTTS/
  voice/       音源ディレクトリ
  models/      韻律モデル
  renderer/    Rendererのmanifest
  runtime/
    utautts-worldline-bridge[.exe]
    utautts-world-engine.dll またはOS用の共有ライブラリ
    utautts-openjtalk-features[.exe]
    open_jtalk_dic_utf_8-1.11/
```

開発環境でhelperや辞書が別の場所にある場合は、`-openjtalk-helper`と`-openjtalk-dictionary`で指定します。音源の配置は`-voice-dir`で変更できます。資源と音源の利用・再配布条件は、それぞれのライセンスに従ってください。
