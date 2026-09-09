"""Import a pinned CMUdict checkout without adding a runtime dependency."""
import gzip
import hashlib
from pathlib import Path
import subprocess
import sys

root = Path(__file__).resolve().parents[1]
source = Path(sys.argv[1])
revision = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
raw = (source / "cmudict.dict").read_bytes()
target = root / "internal/frontend/lexicon"
target.mkdir(parents=True, exist_ok=True)
# Keep the original bytes, including alternative pronunciations and stress.
(target / "cmudict.dict.gz").write_bytes(gzip.compress(raw, mtime=0))
(target / "LICENSE").write_bytes((source / "LICENSE").read_bytes())
(target / "README.md").write_text(
    "# 同梱英語発音辞書\n\n"
    "出典: https://github.com/cmusphinx/cmudict\n\n"
    f"Revision: `{revision}`\n\n"
    f"展開後のSHA-256: `{hashlib.sha256(raw).hexdigest()}`\n\n"
    "`cmudict.dict.gz`は原文を変更せずgzipで圧縮した辞書です。"
    "実行時は基本の発音を選び母音の強勢を保持します。Goバイナリへ埋め込むためビルド時や実行時のダウンロードは不要です。\n\n"
    "利用条件は[LICENSE](LICENSE)と配布物の`licenses/Go/CMUDICT-LICENSE.txt`を参照してください。\n\n"
    "更新時はCMUdictのリポジトリを取得して使用するrevisionへ切り替えます。"
    "UtauTTSのルートから`python tools/import-cmudict.py <checkout>`を実行してください。"
    "`<checkout>`はCMUdictの作業ディレクトリです。辞書とライセンスに加えてこの出典情報も更新します。\n", encoding="utf-8")
