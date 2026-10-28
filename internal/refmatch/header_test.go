package refmatch

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadHeaderWithoutPayload(t *testing.T) {
	header := Header{Version: Version(), Model: "test-model", Source: filepath.Join(t.TempDir(), "bank"), Frames: 1000000, Dims: 768, APBands: 5}
	data, err := json.Marshal(header)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "header.idx")
	contents := append([]byte(indexMagic), make([]byte, 4)...)
	binary.LittleEndian.PutUint32(contents[4:], uint32(len(data)))
	contents = append(contents, data...)
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIndexInfo(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != header.Model || got.Source != header.Source || got.Version != header.Version || got.Path != path || got.Name != "header.idx" || got.SizeBytes != int64(len(contents)) || got.ModifiedMS <= 0 {
		t.Fatalf("header=%+v", got)
	}
}
