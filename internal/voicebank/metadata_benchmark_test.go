package voicebank

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var benchmarkBank *Bank

func metadataFixture(b *testing.B, large bool) string {
	b.Helper()
	root := b.TempDir()
	files := map[string]string{
		"oto.ini":        "a.wav=あ,0,0,0,0,0\n",
		"character.yaml": "default_phonemizer: ja-kana\nsubbanks:\n- color: \"\"\n  prefix: \"\"\n  suffix: \" C4\"\n  tone_ranges: [C3-C5]\n",
		"character.txt":  "name=テスト音源\n",
		"presamp.ini":    "[VOWEL]\na=a=あ,い=100\n",
		"arpasing.yaml":  "entries: []\n",
	}
	var prefix strings.Builder
	for i := 0; i < 1024; i++ {
		fmt.Fprintf(&prefix, "note%04d\t前%d\t 後%d\n", i, i, i)
	}
	files["prefix.map"] = prefix.String()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
			b.Fatal(err)
		}
	}
	if large {
		for i := 0; i < 256; i++ {
			if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("recording-%04d.wav", i)), nil, 0600); err != nil {
				b.Fatal(err)
			}
		}
	}
	return root
}

func BenchmarkPrefixMapMetadata(b *testing.B) {
	root := metadataFixture(b, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bank := &Bank{Root: root, PrefixMap: map[string]Affix{}}
		bank.loadMetadata()
		benchmarkBank = bank
	}
}

func BenchmarkBankMetadata(b *testing.B) {
	root := metadataFixture(b, true)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		benchmarkBank, err = Load(root)
		if err != nil {
			b.Fatal(err)
		}
	}
}
