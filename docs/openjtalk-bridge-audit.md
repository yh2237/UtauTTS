# Open JTalk 実行時ブリッジの移行判定

`internal/openjtalk/frontend_exec.go` は `utautts-openjtalk-features` を起動し、辞書パスと文章を渡して `Analysis` の JSON を受け取る。ブリッジは `tools/openjtalk-feature-bridge.py` と `tools/openjtalk_feature_common.py` を使う。`tools/openjtalk_features.py` は同じ特徴変換を pyopenjtalk から呼ぶ検証用入口であり、`tools/verify-openjtalk-feature-bridge.py` は同梱実行ファイルを検査する。

| 必要な機能 | Python ブリッジが行う処理 | Go 実装の状態 |
| --- | --- | --- |
| 文章から NJD ノードを生成 | 辞書を指定して `openjtalk.OpenJTalk(...).run_frontend(text)` を呼び、単語境界、品詞、活用、読み、発音、アクセント、連結情報を取得 | **ネイティブ実行では未実装。** `frontend_exec.go` は依然としてブリッジを呼ぶ。`frontend_wasm.go` はブラウザの Open JTalk WASM/C 実装から TSV を受け取る。 |
| 読みとモーラ列 | ノードの発音を連結し、句読点、促音、小書き仮名、長音、休止を処理 | `features.go` の `parseNJD`、`analyzeNJD`、モーラ処理で実装済み。NJD 入力が必要。 |
| アクセント句の補正 | 「て／で」後の補助動詞とサ変動詞を連結し、話題助詞の前へアクセント核を置く | `features.go` の `refineAccentPhrases` で実装済み。 |
| モーラごとのアクセント特徴 | 句内位置、句長、核位置、高低、句頭／句末、語頭／語末を算出 | `features.go` の `moraToken.sparseFeatures` で実装済み。 |
| 品詞の疎な特徴 | `pos=*`、`pos_group1=*`、`accent_type=heiban/before/nucleus/after` を出力 | `features.go` で実装済み。 |
| 実行時プロトコル | `--dictionary` と `--serve`、改行区切り JSON、エラー応答、複数リクエストを処理 | `helper_client.go` は**クライアント**のみ。ブリッジの代わりになる Go サーバー／実行ファイルはない。 |
| リリース同梱 | Windows/Linux/macOS で PyInstaller 実行ファイル、Open JTalk 辞書、Python・PyInstaller のライセンスを同梱 | 3 OS のリリーススクリプトは現在のブリッジを同梱する。Go への置換は未実施。 |

pyopenjtalk 0.4.1 とその辞書が利用可能な環境で、5 文（挨拶、天気、図書館、助動詞を含む文、長音と促音を含む文）について `tools/openjtalk-feature-bridge.py` の実際の JSON 応答と Python の `openjtalk_feature_common.analyze` / `sparse_features` の出力を比較した。さらに同じ NJD ノードを入力した Go の `buildAnalysis` と比較した。読み、モーラ列、全特徴が一致した。再現用の小さな入力と期待値は [`internal/openjtalk/testdata/bridge_parity.json`](../internal/openjtalk/testdata/bridge_parity.json) にあり、`go test ./internal/openjtalk -run TestCapturedPyopenjtalkBridgeParity` で検査できる。

この一致は **NJD ノード以降**の比較である。ネイティブ Go 実行が辞書から NJD ノードを作れないため、任意の文章に対するブリッジの置換条件は満たしていない。ブリッジ本体、検証スクリプト、3 OS のビルドスクリプト、`tools/collect-pyinstaller-runtime-licenses.py` を残す。置換には、ネイティブの辞書駆動 Open JTalk フロントエンドと同じ JSON サービス契約を Go から提供し、同じ文章群でブリッジとエンドツーエンドの出力を比較する必要がある。
