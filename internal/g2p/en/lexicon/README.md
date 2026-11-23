# 同梱英語発音辞書

## CMUdict

出典: https://github.com/cmusphinx/cmudict

Revision: `74790861f652b15e4ac49015a90074ad62a27690`

展開後のSHA-256: `286782f036cd341701058943e0615475c8d543ff1ead5b9ddaafd095a352d66a`

`cmudict.dict.gz` は原文を変更せずgzip圧縮した辞書です。実行時は基本の発音を選び、母音の強勢を保持します。Goバイナリへ埋め込むため、ビルド時や実行時のダウンロードは不要です。

利用条件は [LICENSE](LICENSE) と配布物の `licenses/Go/CMUDICT-LICENSE.txt` を参照してください。

更新時はCMUdictリポジトリを取得して対象のrevisionへ切り替え、UtauTTSのルートから `go run ./cmd/tools/import-cmudict <checkout>`（`<checkout>` はCMUdictの作業ディレクトリ）を実行してください。辞書とライセンスに加え、この出典情報も更新します。

## LibriSpeech

`librispeech.dict.gz` は、OpenSLR SLR11のLibriSpeech発音辞書（パブリックドメイン）からCMUdictにない語だけを抽出した補助辞書です。出典・抽出条件・SHA-256は `licenses/LIBRISPEECH-LEXICON-NOTICE.txt` にあります。CMUdictにある語は常にCMUdictを優先します。
