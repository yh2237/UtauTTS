package frontend

import "testing"

func TestPhoneEnglishWordCodas(t *testing.T) {
	for name, parse := range map[string]func(string, string, map[string]string) (string, []Mora, error){"arpasing": ParseEnglishARPAsing, "cv": ParseEnglishCV} {
		t.Run(name, func(t *testing.T) {
			_, m, err := parse("", "T EH1 S T | R IY1 D | SP", nil)
			if err != nil {
				t.Fatal(err)
			}
			for i, role := range []string{"onset", "nucleus", "coda", "coda", "onset", "nucleus", "coda"} {
				if m[i].Phones[0].Role != role {
					t.Fatalf("phone %d: got %s, want %s", i, m[i].Phones[0].Role, role)
				}
			}
			if len(m[6].Aliases.EndingPhones) != 0 {
				t.Fatal("optional release became required")
			}
		})
	}
}

func TestChineseNasalCodaPreservesAliases(t *testing.T) {
	for _, tc := range []struct{ reading, nucleus, coda string }{{"ban1", "a", "n"}, {"bang1", "a", "ng"}, {"jin1", "i", "n"}, {"jing1", "i", "ng"}, {"ma1", "a", ""}} {
		_, m, err := ParseChineseCVVC("", tc.reading, nil)
		if err != nil {
			t.Fatal(err)
		}
		phones := m[0].Phones
		if phones[1].Symbol != tc.nucleus || phones[1].Role != "nucleus" {
			t.Fatal(tc.reading, phones)
		}
		if tc.coda != "" && (len(phones) != 3 || phones[2].Symbol != tc.coda || phones[2].Role != "coda") {
			t.Fatal(tc.reading, phones)
		}
		if tc.coda == "" && len(phones) != 2 {
			t.Fatal(tc.reading, phones)
		}
		if len(m[0].Aliases.EndingPhones) != 0 {
			t.Fatal("nasal duplicated in ending")
		}
	}
}
