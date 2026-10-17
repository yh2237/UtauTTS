package oto

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIniAPIsPreserveSourcesDuplicatesAndDiagnostics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oto.ini")
	data := []byte("\ufeffsub\\a.wav=あ,12.5,100.25,-20.5,30.75,10.5\nsub\\a.wav=あ,0,0,0,0,0\nbroken\n")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := ReadIni(path)
	if err != nil {
		t.Fatal(err)
	}
	text, encoding, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{
		{Filename: filepath.Join(filepath.Dir(path), "sub", "a.wav"), Alias: "あ", Offset: 12.5,
			Fixed: 100.25, Blank: -20.5, Preutterance: 30.75, Overlap: 10.5, OtoPath: path, Line: 1},
		{Filename: filepath.Join(filepath.Dir(path), "sub", "a.wav"), Alias: "あ", OtoPath: path, Line: 2},
	}
	if file.Path != path || file.Encoding != "UTF-8" || !reflect.DeepEqual(file.Entries["あ"], want) {
		t.Fatalf("mapped ini = %+v", file)
	}
	if len(file.Diagnostics) != 1 || file.Diagnostics[0].Line != 3 || file.Diagnostics[0].Message == "" {
		t.Fatalf("diagnostics = %+v", file.Diagnostics)
	}
	virtual, err := parseText(text, encoding, "uploaded/oto.ini", "recordings")
	if err != nil || virtual.Path != "uploaded/oto.ini" || virtual.Entries["あ"][0].Filename != filepath.Join("recordings", "sub", "a.wav") {
		t.Fatalf("virtual source path = %+v, %v", virtual, err)
	}
}
