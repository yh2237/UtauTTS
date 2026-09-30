package voicebank

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var benchmarkPresamp *Presamp

func BenchmarkLoadPresamp(b *testing.B) {
	var text strings.Builder
	text.WriteString("[VOWEL]\n")
	for i := 0; i < 256; i++ {
		fmt.Fprintf(&text, "v%d=v%d=音%d, 音%d_別,音%d_extra=100\n", i, i, i, i, i)
	}
	text.WriteString("[CONSONANT]\n")
	for i := 0; i < 256; i++ {
		fmt.Fprintf(&text, "c%d=音%d,音%d_別=0\n", i, i, i)
	}
	text.WriteString("[REPLACE]\nlu:=lv\n[ENDTYPE1]\n%v% R\n[ENDTYPE2]\n-\n[ENDFLAG]\n1\n")
	root := b.TempDir()
	if err := os.WriteFile(filepath.Join(root, "presamp.ini"), []byte(text.String()), 0600); err != nil {
		b.Fatal(err)
	}
	bank := &Bank{Root: root}
	bank.loadPresamp()
	if bank.Presamp == nil || len(bank.Presamp.Vowels) != 768 || len(bank.Presamp.Consonants) != 512 {
		b.Fatal("invalid fixture")
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bank := &Bank{Root: root}
		bank.loadPresamp()
		benchmarkPresamp = bank.Presamp
	}
}
