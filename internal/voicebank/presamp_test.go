package voicebank

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPresamp(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "presamp.ini")
	content := "[VOWEL]\nir=ir=zhi,chi,shi,ri=100\nao=ao=hao=100\n" +
		"[CONSONANT]\nzh=zhi=0\nh=hao=0\n[REPLACE]\nlu:=lv\n[ENDTYPE1]\n%v% R\n[ENDTYPE2]\n-\n[ENDFLAG]\n1\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	bank := &Bank{Root: root}
	bank.loadPresamp()
	if bank.Presamp == nil || bank.Presamp.Vowels["zhi"] != "ir" || bank.Presamp.Consonants["hao"] != "h" {
		t.Fatalf("presamp=%#v", bank.Presamp)
	}
	if bank.Presamp.Replacements["lu:"] != "lv" || len(bank.Presamp.Endings) != 1 || bank.Presamp.Endings[0] != "%v% R" {
		t.Fatalf("presamp=%#v", bank.Presamp)
	}
}
