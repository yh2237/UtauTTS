package native

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTakeFloat32FileReadsAndRemovesInferenceResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "read-0.f32")
	data := make([]byte, 8)
	binary.LittleEndian.PutUint32(data[0:], math.Float32bits(0.5))
	binary.LittleEndian.PutUint32(data[4:], math.Float32bits(-0.25))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	values, err := takeFloat32FileIn(dir, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0] != 0.5 || values[1] != -0.25 {
		t.Fatalf("values = %v", values)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("the inference result was not removed")
	}
}

func TestTakeFloat32FileRejectsPathsOutsideInferenceDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "outside.f32")
	if err := os.WriteFile(path, make([]byte, 4), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := takeFloat32FileIn(filepath.Join(dir, "inference"), path); err == nil {
		t.Fatal("read a file outside the inference directory")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the rejected file was removed")
	}
	if _, err := takeFloat32FileIn(filepath.Join(dir, "inference"), filepath.Join(dir, "inference", "..", "outside.f32")); err == nil {
		t.Fatal("accepted a path that escapes the inference directory")
	}
}

// On Unix a backslash is a literal filename character, so a path that the
// slash-normalized check sees as inside the directory can still resolve
// outside it. The read and delete must use the validated path.
func TestTakeFloat32FileRejectsBackslashEscapeOnUnix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("backslash is a path separator on Windows")
	}
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	// A real directory whose name contains a literal backslash.
	if err := os.MkdirAll(filepath.Join(root, `inside\sub`), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.f32")
	if err := os.WriteFile(outside, make([]byte, 4), 0o644); err != nil {
		t.Fatal(err)
	}
	candidate := root + `/inside\sub/../outside.f32`
	if _, err := takeFloat32FileIn(inside, candidate); err == nil {
		t.Fatal("accepted a path that escapes the inference directory")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal("the rejected file was removed")
	}
}
