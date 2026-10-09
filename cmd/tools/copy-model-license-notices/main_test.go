package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyAndCheckNotices(t *testing.T) {
	repository := t.TempDir()
	writeFile(t, filepath.Join(repository, "licenses", "MODEL-NOTICE.txt"), "notice body")
	models := filepath.Join(repository, "models")
	writeFile(t, filepath.Join(models, "voice-v1.json"), `{"license":"CC0","license_notices":["licenses/MODEL-NOTICE.txt"]}`)

	packageRoot := t.TempDir()
	if err := copyNotices(models, "", packageRoot, repository, false); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(packageRoot, "licenses", "MODEL-NOTICE.txt")
	data, err := os.ReadFile(copied)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "notice body" {
		t.Fatalf("unexpected content: %q", data)
	}
	if err := copyNotices(models, "", packageRoot, "", true); err != nil {
		t.Fatal(err)
	}
	if err := copyNotices(models, "", t.TempDir(), "", true); err == nil || !strings.Contains(err.Error(), "packaged license notice is missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopyNoticesIncludesEmbeddedModels(t *testing.T) {
	repository := t.TempDir()
	writeFile(t, filepath.Join(repository, "licenses", "MODEL-NOTICE.txt"), "notice body")
	writeFile(t, filepath.Join(repository, "licenses", "EMBEDDED-NOTICE.txt"), "embedded body")
	models := filepath.Join(repository, "models")
	writeFile(t, filepath.Join(models, "voice-v1.json"), `{"license":"CC0","license_notices":["licenses/MODEL-NOTICE.txt"]}`)
	embedded := filepath.Join(repository, "embedded")
	header := `{"__metadata__":{"license":"CC BY 4.0","license_notices":"licenses/EMBEDDED-NOTICE.txt licenses/MODEL-NOTICE.txt"}}`
	size := []byte{byte(len(header)), 0, 0, 0, 0, 0, 0, 0}
	writeFile(t, filepath.Join(embedded, "timing-v1.safetensors"), string(size)+header)

	packageRoot := t.TempDir()
	if err := copyNotices(models, embedded, packageRoot, repository, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(packageRoot, "licenses", "EMBEDDED-NOTICE.txt")); err != nil {
		t.Fatal(err)
	}
	if err := copyNotices(models, embedded, packageRoot, "", true); err != nil {
		t.Fatal(err)
	}
	missing := t.TempDir()
	writeFile(t, filepath.Join(missing, "licenses", "MODEL-NOTICE.txt"), "notice body")
	if err := copyNotices(models, embedded, missing, "", true); err == nil || !strings.Contains(err.Error(), "packaged license notice is missing") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopyNoticesAcceptsLegacyField(t *testing.T) {
	repository := t.TempDir()
	writeFile(t, filepath.Join(repository, "licenses", "LEGACY.txt"), "legacy")
	models := filepath.Join(repository, "models")
	writeFile(t, filepath.Join(models, "voice-v1.json"), `{"license":"CC0","license_notice":"licenses/LEGACY.txt"}`)
	packageRoot := t.TempDir()
	if err := copyNotices(models, "", packageRoot, repository, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(packageRoot, "licenses", "LEGACY.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestCopyNoticesRejectsEscapingPath(t *testing.T) {
	repository := t.TempDir()
	writeFile(t, filepath.Join(repository, "licenses", "MODEL-NOTICE.txt"), "notice")
	writeFile(t, filepath.Join(repository, "secret.txt"), "secret")
	models := filepath.Join(repository, "models")
	writeFile(t, filepath.Join(models, "voice-v1.json"), `{"license":"CC0","license_notices":["licenses/../secret.txt"]}`)
	err := copyNotices(models, "", t.TempDir(), repository, false)
	if err == nil || !strings.Contains(err.Error(), "normalized paths below licenses/") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCopyNoticesRequiresMetadata(t *testing.T) {
	missingNotices := t.TempDir()
	writeFile(t, filepath.Join(missingNotices, "voice-v1.json"), `{"license":"CC0"}`)
	err := copyNotices(missingNotices, "", t.TempDir(), t.TempDir(), false)
	if err == nil || !strings.Contains(err.Error(), "license_notices is required") {
		t.Fatalf("unexpected error: %v", err)
	}
	missingLicense := t.TempDir()
	writeFile(t, filepath.Join(missingLicense, "voice-v1.json"), `{"license_notices":["licenses/A.txt"]}`)
	err = copyNotices(missingLicense, "", t.TempDir(), t.TempDir(), false)
	if err == nil || !strings.Contains(err.Error(), "license is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseNoticePath(t *testing.T) {
	valid := []string{"licenses/A.txt", "licenses/openjtalk/LICENSE.txt", "licenses/a/b-c_d.txt"}
	for _, value := range valid {
		if _, err := parseNoticePath(value); err != nil {
			t.Fatalf("%q: %v", value, err)
		}
	}
	invalid := []string{
		"", " ", "licenses", "licenses/", "licenses//A.txt", "licenses/./A.txt",
		"licenses/../A.txt", "/licenses/A.txt", "models/A.txt", `licenses\A.txt`,
		"licenses/A.txt ", " licenses/A.txt", "licenses/A:B.txt",
	}
	for _, value := range invalid {
		if value == "" || strings.TrimSpace(value) == "" {
			if _, err := parseNoticePath(value); err == nil {
				t.Fatalf("%q: expected an error", value)
			}
			continue
		}
		if _, err := parseNoticePath(value); err == nil || !strings.Contains(err.Error(), "normalized paths below licenses/") {
			t.Fatalf("%q: unexpected error: %v", value, err)
		}
	}
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
