# 韻律研究用スクリプト

ここにあるスクリプトは、通常のビルド・リリースやアプリ実行からは呼び出されません。
現行の配布モデルを再生成する経路は `tools/` に残し、診断や旧モデルの再現に使うものだけをここへ置きます。

- `audit-jsut-label-alignment.py`: JSUTラベルのモーラ対応率を調べる
- `audit-openjtalk-alignment.py`: Open JTalkとのアラインメントを調べる
- `openjtalk-accent-contours.py`: Open JTalkアクセントの診断用輪郭を出力する
- `legacy/train-intonation-tcn.py`: 現行のフレームモデル以前のモーラ単位モデルを学習する
