package oto

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding/japanese"
)

func TestParseBytesMatchesFileEntryPoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oto.ini")
	data := []byte("\ufeffa.wav=あ,1,2,-3,4,5\na.wav=あ,6,7,-8,9,10\nbroken\n")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := ReadIni(path)
	if err != nil {
		t.Fatal(err)
	}
	memory, err := ParseBytes(data, path, filepath.Dir(path))
	if err != nil || !reflect.DeepEqual(file, memory) {
		t.Fatalf("file and byte APIs differ: %v, %v", memory, err)
	}
}

func TestParseBytesUTF16BEAndOddLength(t *testing.T) {
	data := []byte{0xfe, 0xff}
	for _, unit := range utf16.Encode([]rune("a.wav=あ,0,0,0,0,0\n")) {
		data = binary.BigEndian.AppendUint16(data, unit)
	}
	ini, err := ParseBytes(data, "memory.ini", "relative")
	if err != nil || ini.Encoding != "UTF-16BE" || ini.Entries["あ"][0].OtoPath != "memory.ini" {
		t.Fatalf("BE parse: %v, %v", ini, err)
	}
	if _, err := ParseBytes(append(data, 0), "memory.ini", "relative"); err == nil {
		t.Fatal("odd UTF-16 data accepted")
	}
}

func TestDecodeUTF16LEWithBOM(t *testing.T) {
	units := utf16.Encode([]rune("音源の説明です。\r\n"))
	data := []byte{0xff, 0xfe}
	for _, unit := range units {
		var encoded [2]byte
		binary.LittleEndian.PutUint16(encoded[:], unit)
		data = append(data, encoded[:]...)
	}
	text, encoding, err := Decode(data)
	if err != nil || encoding != "UTF-16LE" || text != "音源の説明です。\r\n" {
		t.Fatalf("Decode() = %q, %q, %v", text, encoding, err)
	}
}

func TestReadIniUTF8DecimalsAndDiagnostics(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oto.ini")
	content := "\ufeffa.wav=あ,12.5,100.25,-20.5,30.75,10.5\n" +
		"plain.wav=,,,,,\n" +
		"broken line\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	ini, err := ReadIni(path)
	if err != nil {
		t.Fatal(err)
	}
	if ini.Encoding != "UTF-8" {
		t.Fatalf("encoding = %q", ini.Encoding)
	}
	entry := ini.Entries["あ"][0]
	if entry.Offset != 12.5 || entry.Fixed != 100.25 || entry.Blank != -20.5 || entry.Preutterance != 30.75 || entry.Overlap != 10.5 {
		t.Fatalf("unexpected parameters: %+v", entry)
	}
	if got := ini.Entries["plain"][0].Alias; got != "plain" {
		t.Fatalf("blank alias fallback = %q", got)
	}
	if got := ini.Entries["plain"][0].Offset; got != 0 {
		t.Fatalf("blank numeric value = %v", got)
	}
	if len(ini.Diagnostics) != 1 || ini.Diagnostics[0].Line != 3 {
		t.Fatalf("diagnostics = %+v", ini.Diagnostics)
	}
}

func TestReadIniShiftJIS(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oto.ini")
	encoded, err := japanese.ShiftJIS.NewEncoder().Bytes([]byte("voice.wav=こんにちは,0,100,-20,30,10\n"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatal(err)
	}

	ini, err := ReadIni(path)
	if err != nil {
		t.Fatal(err)
	}
	if ini.Encoding != "Shift_JIS" {
		t.Fatalf("encoding = %q", ini.Encoding)
	}
	if len(ini.Entries["こんにちは"]) != 1 {
		t.Fatalf("entries = %#v", ini.Entries)
	}
}

func TestReadIniNormalizesWindowsRelativePath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oto.ini")
	if err := os.WriteFile(path, []byte("sub\\a.wav=あ,0,0,0,0,0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ini, err := ReadIni(path)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "sub", "a.wav")
	if got := ini.Entries["あ"][0].Filename; got != want {
		t.Fatalf("filename = %q, want %q", got, want)
	}
}

func TestReadIniRejectsNonFiniteParameters(t *testing.T) {
	dir := t.TempDir()
	for _, value := range []string{"NaN", "Inf", "-Inf"} {
		path := filepath.Join(dir, value+".ini")
		if err := os.WriteFile(path, []byte("a.wav=alias,"+value+",0,0,0,0\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		ini, err := ReadIni(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(ini.Entries) != 0 || len(ini.Diagnostics) != 1 {
			t.Fatalf("accepted non-finite parameter %q: %#v", value, ini)
		}
	}
}
