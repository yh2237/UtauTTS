# 同梱英語発音辞書

出典: https://github.com/cmusphinx/cmudict

Revision: `74790861f652b15e4ac49015a90074ad62a27690`

展開後のSHA-256: `286782f036cd341701058943e0615475c8d543ff1ead5b9ddaafd095a352d66a`

`cmudict.dict.gz`は原文を変更せずgzipで圧縮した辞書です。実行時は基本の発音を選び母音の強勢を保持します。Goバイナリへ埋め込むためビルド時や実行時のダウンロードは不要です。

利用条件は[LICENSE](LICENSE)と配布物の`licenses/Go/CMUDICT-LICENSE.txt`を参照してください。

更新時はCMUdictのリポジトリを取得して使用するrevisionへ切り替えます。UtauTTSのルートから`python tools/import-cmudict.py <checkout>`を実行してください。`<checkout>`はCMUdictの作業ディレクトリです。辞書とライセンスに加えてこの出典情報も更新します。
