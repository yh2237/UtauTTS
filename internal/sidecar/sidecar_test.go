package sidecar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestWriteSidecars(t *testing.T) {
	t.Run("UTF-8", testWriteCreatesMatchingUTF8Sidecars)
	t.Run("Shift_JIS", testWriteEncodesShiftJIS)
}

func testWriteCreatesMatchingUTF8Sidecars(t *testing.T) {
	directory := t.TempDir()
	wav := filepath.Join(directory, "voice.wav")
	if err := Write(wav, Options{WriteText: true, WriteLab: true, Encoding: EncodingUTF8,
		Text: "こんにちは", Lab: "0 1000000 k\n1000000 2000000 o\n"}); err != nil {
		t.Fatal(err)
	}
	textData, err := os.ReadFile(filepath.Join(directory, "voice.txt"))
	if err != nil || string(textData) != "こんにちは\n" {
		t.Fatalf("unexpected text sidecar: %q, %v", textData, err)
	}
	labData, err := os.ReadFile(filepath.Join(directory, "voice.lab"))
	if err != nil || !strings.HasSuffix(string(labData), "2000000 o\n") {
		t.Fatalf("unexpected label sidecar: %q, %v", labData, err)
	}
}

func testWriteEncodesShiftJIS(t *testing.T) {
	directory := t.TempDir()
	if err := Write(filepath.Join(directory, "voice.wav"), Options{
		WriteText: true, Encoding: EncodingShiftJIS, Text: "足立レイ",
	}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "voice.txt"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, _, err := transform.Bytes(japanese.ShiftJIS.NewDecoder(), data)
	if err != nil || string(decoded) != "足立レイ\n" {
		t.Fatalf("unexpected Shift_JIS text: %q, %v", decoded, err)
	}
}
