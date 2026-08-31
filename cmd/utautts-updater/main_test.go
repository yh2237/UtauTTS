package main

import (
	"archive/zip"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeZip(t *testing.T, files map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.zip")
	archive, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExtractZip(t *testing.T) {
	zipPath := makeZip(t, map[string]string{
		"app/utautts-gui.exe": "binary",
		"app/qml/Main.qml":    "main",
		"models/prosody.json": "model",
		"README.md":           "readme",
	})
	dest := filepath.Join(t.TempDir(), "stage")
	if err := extractZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"app/utautts-gui.exe",
		"app/qml/Main.qml",
		"models/prosody.json",
		"README.md",
	} {
		if _, err := os.Stat(filepath.Join(dest, path)); err != nil {
			t.Errorf("missing extracted entry %s: %v", path, err)
		}
	}
}

func TestExtractZipWindowsBackslashSeparators(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "test.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for _, entry := range []struct{ name, content string }{
		{"app\\utautts-gui.exe", "binary"},
		{"app\\qml\\Main.qml", "main"},
		{"app\\qml\\EditorContent.qml", "editor"},
		{"models\\prosody.json", "model"},
		{"README.md", "readme"},
	} {
		created, err := writer.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := created.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	_ = archive.Close()

	dest := filepath.Join(t.TempDir(), "stage")
	if err := extractZip(zipPath, dest); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"app/utautts-gui.exe",
		"app/qml/Main.qml",
		"app/qml/EditorContent.qml",
		"models/prosody.json",
		"README.md",
	} {
		if _, err := os.Stat(filepath.Join(dest, path)); err != nil {
			t.Errorf("missing extracted entry %s: %v", path, err)
		}
	}
	if info, err := os.Stat(filepath.Join(dest, "app/qml")); err != nil || !info.IsDir() {
		t.Errorf("app/qml should be a directory, err=%v", err)
	}
}

func TestExtractZipPreservesExecutableMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX executable mode bits")
	}
	zipPath := filepath.Join(t.TempDir(), "linux.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	header := &zip.FileHeader{Name: "utautts", Method: zip.Deflate}
	header.SetMode(0o755)
	entry, err := writer.CreateHeader(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte("binary")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	destination := t.TempDir()
	if err := extractZip(zipPath, destination); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(destination, "utautts"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("executable mode was lost: %v", info.Mode().Perm())
	}
}

func TestRunUpdatesLinuxPackageAndKeepsExecutableMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Linux package mode test")
	}
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	writeTestFile(t, filepath.Join(target, "utautts"), "old")
	writeTestFile(t, filepath.Join(target, "voice", "bank", "oto.ini"), "voice-data")

	zipPath := filepath.Join(root, "linux-update.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for _, item := range []struct {
		name    string
		content string
		mode    os.FileMode
	}{
		{name: "utautts", content: "new", mode: 0o755},
		{name: "README.md", content: "readme", mode: 0o644},
	} {
		header := &zip.FileHeader{Name: item.name, Method: zip.Deflate}
		header.SetMode(item.mode)
		entry, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(item.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	if err := run(target, "", zipPath, 0, "test", []string{"voice"}, false); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(target, "utautts"))
	if err != nil || string(content) != "new" {
		t.Fatalf("updated GUI = %q, %v", content, err)
	}
	info, err := os.Stat(filepath.Join(target, "utautts"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Fatalf("updated GUI is not executable: %v", info.Mode().Perm())
	}
	voice, err := os.ReadFile(filepath.Join(target, "voice", "bank", "oto.ini"))
	if err != nil || string(voice) != "voice-data" {
		t.Fatalf("voice data was not preserved: %q, %v", voice, err)
	}
}

func TestExtractZipReplacesFileDirCollision(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "test.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for _, entry := range []struct{ name, content string }{
		{"app/utautts-gui.exe", "binary"},
		{"app/qml", ""},
		{"app/qml/Main.qml", "main"},
		{"app/qml/EditorContent.qml", "editor"},
	} {
		file, err := writer.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "stage")
	if err := extractZip(zipPath, dest); err != nil {
		t.Fatalf("extractZip failed on file/dir collision: %v", err)
	}
	for _, path := range []string{"app/utautts-gui.exe", "app/qml/Main.qml", "app/qml/EditorContent.qml"} {
		if info, err := os.Stat(filepath.Join(dest, path)); err != nil {
			t.Errorf("missing extracted entry %s: %v", path, err)
		} else if info.IsDir() {
			t.Errorf("entry %s should be a file", path)
		}
	}
	if info, err := os.Stat(filepath.Join(dest, "app", "qml")); err != nil {
		t.Errorf("qml should exist as a directory: %v", err)
	} else if !info.IsDir() {
		t.Error("qml should be a directory after extraction")
	}
}

func TestExtractZipRejectsPathTraversal(t *testing.T) {
	zipPath := makeZip(t, map[string]string{
		"../escape.txt": "boom",
	})
	if err := extractZip(zipPath, filepath.Join(t.TempDir(), "stage")); err == nil {
		t.Fatal("expected path traversal rejection")
	}
}

func TestNormalizeStageMovesTopLevelFolder(t *testing.T) {
	root := t.TempDir()
	stage := filepath.Join(root, "stage")
	writeTestFile(t, filepath.Join(stage, "UtauTTS", "app", "utautts-gui.exe"), "binary")
	writeTestFile(t, filepath.Join(stage, "UtauTTS", "voice", "bundle", "oto.ini"), "oto")
	if err := normalizeStage(stage); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(stage, "app", "utautts-gui.exe")); err != nil {
		t.Errorf("top-level folder was not moved up: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stage, "voice", "bundle", "oto.ini")); err != nil {
		t.Errorf("nested content was not preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(stage, "UtauTTS")); !os.IsNotExist(err) {
		t.Errorf("top-level folder should have been removed")
	}
}

func TestNormalizeStageFlatLayoutIsValid(t *testing.T) {
	stage := filepath.Join(t.TempDir(), "stage")
	writeTestFile(t, filepath.Join(stage, "utautts.exe"), "launcher")
	if err := normalizeStage(stage); err != nil {
		t.Fatalf("flat package layout should be accepted: %v", err)
	}
}

func TestNormalizeStageRejectsUnknownLayout(t *testing.T) {
	stage := filepath.Join(t.TempDir(), "stage")
	writeTestFile(t, filepath.Join(stage, "random.txt"), "junk")
	if err := normalizeStage(stage); err == nil {
		t.Fatal("expected unknown layout to be rejected")
	}
}

func TestCopyTree(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	destination := filepath.Join(t.TempDir(), "destination")
	writeTestFile(t, filepath.Join(source, "voice", "bank-a", "oto.ini"), "oto")
	writeTestFile(t, filepath.Join(source, "voice", "bank-a", "wav", "a.wav"), "audio")
	writeTestFile(t, filepath.Join(source, "voice", "bank-b", "oto.ini"), "oto2")
	if err := copyTree(source, destination); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"voice/bank-a/oto.ini",
		"voice/bank-a/wav/a.wav",
		"voice/bank-b/oto.ini",
	} {
		content, err := os.ReadFile(filepath.Join(destination, path))
		if err != nil {
			t.Errorf("missing copied file %s: %v", path, err)
			continue
		}
		if len(content) == 0 {
			t.Errorf("copied file %s is empty", path)
		}
	}
}

func TestSanitizeToken(t *testing.T) {
	cases := map[string]string{
		"v0.0.6": "v0.0.6",
		"":       "latest",
		"v 1.0!": "v_1.0_",
		"rc-1_b": "rc-1_b",
	}
	for input, expected := range cases {
		if got := sanitizeToken(input); got != expected {
			t.Errorf("sanitizeToken(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestRunSwapsPackagePreservingVoice(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	writeTestFile(t, filepath.Join(target, "app", "utautts-gui.exe"), "old-gui")
	writeTestFile(t, filepath.Join(target, "utautts.exe"), "old-launcher")
	writeTestFile(t, filepath.Join(target, "voice", "bank-a", "oto.ini"), "voice-data")

	zipPath := filepath.Join(root, "update.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for _, entry := range []struct{ name, content string }{
		{"utautts.exe", "new-launcher"},
		{"app/utautts-gui.exe", "new-gui"},
		{"app/qml", ""},
		{"app/qml/Main.qml", "new-main"},
		{"models/prosody.json", "new-model"},
		{"voice/bundle/oto.ini", "bundled-voice"},
	} {
		file, err := writer.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	if err := run(target, "", zipPath, 0, "v0.0.6", []string{"voice"}, false); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	for _, path := range []string{"utautts.exe", "app/utautts-gui.exe", "app/qml/Main.qml", "models/prosody.json"} {
		if _, err := os.Stat(filepath.Join(target, path)); err != nil {
			t.Errorf("new install missing %s: %v", path, err)
		}
	}
	if content, err := os.ReadFile(filepath.Join(target, "voice", "bank-a", "oto.ini")); err != nil || string(content) != "voice-data" {
		t.Errorf("user voice data was not preserved: %q, %v", content, err)
	}
	if _, err := os.Stat(target + ".stage"); !os.IsNotExist(err) {
		t.Error("staging directory should be removed after a successful update")
	}
	if _, err := os.Stat(target + ".old"); !os.IsNotExist(err) {
		t.Error("backup directory should be removed after a successful update")
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Errorf("caller-owned local archive should be retained: %v", err)
	}
}

func TestRunWorksWhenCWDInsideTarget(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	writeTestFile(t, filepath.Join(target, "app", "utautts-gui.exe"), "old-gui")
	writeTestFile(t, filepath.Join(target, "voice", "bank", "oto.ini"), "voice-data")

	zipPath := filepath.Join(root, "update.zip")
	archive, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(archive)
	for _, entry := range []struct{ name, content string }{
		{"utautts.exe", "new-launcher"},
		{"app/utautts-gui.exe", "new-gui"},
		{"app/qml", ""},
		{"app/qml/Main.qml", "new-main"},
	} {
		file, err := writer.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}

	previousCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(previousCWD)
	if err := os.Chdir(target); err != nil {
		t.Fatal(err)
	}
	if err := run(target, "", zipPath, 0, "v0.0.6", []string{"voice"}, false); err != nil {
		t.Fatalf("run failed while the working directory is inside the target: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "app", "qml", "Main.qml")); err != nil {
		t.Errorf("new install missing app/qml/Main.qml: %v", err)
	}
	if content, err := os.ReadFile(filepath.Join(target, "voice", "bank", "oto.ini")); err != nil || string(content) != "voice-data" {
		t.Errorf("user voice data was not preserved: %q, %v", content, err)
	}
}

func TestSafePreservePathRejectsEscapes(t *testing.T) {
	for _, value := range []string{"", ".", "..", `..\outside`, `C:\outside`} {
		if _, err := safePreservePath(value); err == nil {
			t.Errorf("safePreservePath(%q) accepted an unsafe path", value)
		}
	}
	if got, err := safePreservePath("voice/bank"); err != nil || got != filepath.Join("voice", "bank") {
		t.Fatalf("safe path = %q, %v", got, err)
	}
}

func TestRunPreservesPortableConfigAndExternalRenderers(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	writeTestFile(t, filepath.Join(target, "app", "utautts-gui.exe"), "old-gui")
	writeTestFile(t, filepath.Join(target, "config.ini"), "portable-settings")
	writeTestFile(t, filepath.Join(target, "plugins", "renderers", "external", "plugin.json"),
		`{"backend":"utau-external-resampler","assets":{"resampler":"tool.exe"}}`)
	writeTestFile(t, filepath.Join(target, "plugins", "renderers", "external", "user-data.txt"), "keep")
	writeTestFile(t, filepath.Join(target, "plugins", "renderers", "builtin", "plugin.json"),
		`{"backend":"waveform","version":"old"}`)

	zipPath := makeZip(t, map[string]string{
		"app/utautts-gui.exe":                    "new-gui",
		"config.ini":                             "package-default",
		"plugins/renderers/external/plugin.json": `{"backend":"waveform"}`,
		"plugins/renderers/builtin/plugin.json":  `{"backend":"waveform","version":"new"}`,
	})
	if err := run(target, "", zipPath, 0, "test", []string{"config.ini"}, false); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"config.ini": "portable-settings",
		"plugins/renderers/external/user-data.txt": "keep",
		"plugins/renderers/builtin/plugin.json":    `{"backend":"waveform","version":"new"}`,
	} {
		data, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
		if err != nil || string(data) != want {
			t.Errorf("%s = %q, %v; want %q", path, data, err, want)
		}
	}
}

func TestRunDeletesApplicationOwnedLocalArchive(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	writeTestFile(t, filepath.Join(target, "app", "utautts-gui.exe"), "old-gui")
	zipPath := makeZip(t, map[string]string{"app/utautts-gui.exe": "new-gui"})
	if err := run(target, "", zipPath, 0, "test", nil, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(zipPath); !os.IsNotExist(err) {
		t.Fatalf("application-owned archive was not deleted: %v", err)
	}
}

func TestRunPreserveFailureDoesNotReplaceInstall(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "UtauTTS")
	writeTestFile(t, filepath.Join(target, "app", "utautts-gui.exe"), "old-gui")
	zipPath := makeZip(t, map[string]string{"app/utautts-gui.exe": "new-gui"})
	if err := run(target, "", zipPath, 0, "test", []string{`..\outside`}, false); err == nil {
		t.Fatal("expected preservation failure")
	}
	content, err := os.ReadFile(filepath.Join(target, "app", "utautts-gui.exe"))
	if err != nil || string(content) != "old-gui" {
		t.Fatalf("current install changed after preservation failure: %q, %v", content, err)
	}
}
