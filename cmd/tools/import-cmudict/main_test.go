package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateWritesDictionaryAndNotices(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	checkout := t.TempDir()
	dictionary := "hello HH AH0 L OW1\n"
	writeFile(t, filepath.Join(checkout, "cmudict.dict"), dictionary)
	writeFile(t, filepath.Join(checkout, "LICENSE"), "license text\n")
	runGit(t, checkout, "init", "-q")
	runGit(t, checkout, "add", ".")
	runGit(t, checkout, "-c", "user.name=test", "-c", "user.email=test@example.com", "commit", "-qm", "fixture")
	revision := runGit(t, checkout, "rev-parse", "HEAD")

	root := t.TempDir()
	if err := generate(checkout, root); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "internal", "frontend", "lexicon")
	compressed, err := os.ReadFile(filepath.Join(target, "cmudict.dict.gz"))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(decompressed) != dictionary {
		t.Fatalf("unexpected dictionary: %q", decompressed)
	}
	license, err := os.ReadFile(filepath.Join(target, "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if string(license) != "license text\n" {
		t.Fatalf("unexpected license: %q", license)
	}
	readme, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(readme)
	if !strings.Contains(text, "Revision: `"+revision+"`") {
		t.Fatalf("README does not record the revision:\n%s", text)
	}
	if !strings.Contains(text, "go run ./cmd/tools/import-cmudict <checkout>") {
		t.Fatalf("README does not record the update command:\n%s", text)
	}
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(stdout.String())
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
