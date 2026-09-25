# Dependencies

DiffSinger Rendererで使う共有vocoderを配置します。`dsconfig.yaml`の`vocoder:`が共有vocoder名を指す場合、`Dependencies/<名前>/`に`vocoder.yaml`と`model.onnx`を置きます。名前は音源の指定と一致させてください。

探索先は、UtauTTSを起動したディレクトリの`Dependencies`、実行ファイルと同じ場所の`Dependencies`、OpenUtauの標準`Dependencies`の順です。