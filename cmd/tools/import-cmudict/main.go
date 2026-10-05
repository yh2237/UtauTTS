// import-cmudictは固定版CMUdictチェックアウトを辞書と出典情報として取り込む。
// 実行時依存を増やさず、原文をgzipで埋め込む。
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "import-cmudict:", err)
		os.Exit(1)
	}
}

func run() error {
	root := flag.String("root", ".", "UtauTTS repository root")
	flag.Parse()
	if flag.NArg() != 1 {
		return errors.New("usage: import-cmudict [--root <UtauTTS root>] <cmudict checkout>")
	}
	return generate(flag.Arg(0), *root)
}

func generate(checkout, root string) error {
	revision, err := commandOutput("git", "-C", checkout, "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("git rev-parse: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(checkout, "cmudict.dict"))
	if err != nil {
		return err
	}
	license, err := os.ReadFile(filepath.Join(checkout, "LICENSE"))
	if err != nil {
		return err
	}
	target := filepath.Join(root, "internal", "frontend", "lexicon")
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := writer.Write(raw); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "cmudict.dict.gz"), compressed.Bytes(), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, "LICENSE"), license, 0o644); err != nil {
		return err
	}
	sum := sha256.Sum256(raw)
	readme := "# 同梱英語発音辞書\n\n" +
		"出典: https://github.com/cmusphinx/cmudict\n\n" +
		"Revision: `" + revision + "`\n\n" +
		"展開後のSHA-256: `" + hex.EncodeToString(sum[:]) + "`\n\n" +
		"`cmudict.dict.gz`は原文を変更せずgzipで圧縮した辞書です。実行時は基本の発音を選び母音の強勢を保持します。Goバイナリへ埋め込むためビルド時や実行時のダウンロードは不要です。\n\n" +
		"利用条件は[LICENSE](LICENSE)と配布物の`licenses/Go/CMUDICT-LICENSE.txt`を参照してください。\n\n" +
		"更新時はCMUdictのリポジトリを取得して使用するrevisionへ切り替えます。" +
		"UtauTTSのルートから`go run ./cmd/tools/import-cmudict <checkout>`を実行してください。" +
		"`<checkout>`はCMUdictの作業ディレクトリです。辞書とライセンスに加えてこの出典情報も更新します。\n"
	return os.WriteFile(filepath.Join(target, "README.md"), []byte(readme), 0o644)
}

func commandOutput(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return "", fmt.Errorf("%w: %s", err, message)
		}
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}
