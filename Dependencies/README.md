# Dependencies

DiffSinger Rendererで使う共有vocoderを配置します。`dsconfig.yaml`の`vocoder:`が共有vocoder名を指す場合、`Dependencies/<名前>/`に`vocoder.yaml`と`model.onnx`を置きます。名前は音源の指定と一致させてください。

探索するのは、UtauTTSの作業ディレクトリと実行ファイルと同じ場所の`Dependencies`、およびOpenUtauの標準`Dependencies`です。OpenUtauで使っているvocoderをそのままコピーできます。

再配布する場合は、各vocoder・モデルのライセンスと再配布条件を確認してください。
