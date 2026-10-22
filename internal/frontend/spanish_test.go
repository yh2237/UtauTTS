package frontend

import (
	"strings"
	"testing"
)

// 期待値はmisakiのEspeakG2P（espeak-ng es）の出力に、音節の区切りを足したもの。
func TestSpanishReadingMatchesKokoroPhonemes(t *testing.T) {
	cases := map[string]string{
		"hola":                  "ˈo.la",
		"¿Qué tal?":             "kˈe tˈal ?",
		"gente":                 "xˈɛn.te",
		"un beso":               "ˈum bˈe.so",
		"un gato":               "ˈuŋ ɡˈa.to",
		"muy bonito":            "mˈuj βo.nˈi.to",
		"la verdad es que no":   "la βeɾ.ðˈað ˈes ke nˈo",
		"veinte":                "bˈAn.te",
		"pingüino":              "pˌiŋ.ɡu.ˈi.no",
		"cliente nuevo":         "kli.ˈɛn.te nwˈe.βo",
		"universidad":           "ˌu.ni.βˌeɾ.si.ðˈad",
		"desafortunadamente":    "dˌe.sa.fˌoɾ.tu.nˈa.ða.mˈen.te",
		"Me duele la cabeza.":   "me dwˈe.le la ka.βˈe.θa .",
		"hielo":                 "jˈe.lo",
		"porque llueve":         "pˌoɾ.ke ʎwˈe.βe",
		"uno, dos, tres":        "ˈu.no , dˈos , tɾˈes",
		"México":                "mˈɛ.xi.ko",
		"El perro corre rápido": "el pˈe.ro kˈo.re rˈa.pi.ðo",
		"examen":                "ek.sˈa.men",
	}
	for text, want := range cases {
		got, err := SpanishReading(text, nil)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

func TestSpanishMoraeAreSyllables(t *testing.T) {
	reading, morae, err := ParseSpanish("Hola, ¿qué tal?", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if reading != "ˈo.la , kˈe tˈal ?" {
		t.Fatalf("reading=%q", reading)
	}
	var texts []string
	for _, mora := range morae {
		texts = append(texts, mora.Text)
	}
	// 文中の読点は休止のモーラになり、文末の記号はモーラにしない。
	if got := strings.Join(texts, " "); got != "o la , ke tal" {
		t.Fatalf("morae=%q", got)
	}
	if !morae[2].Pause || morae[0].Stress != 1 || morae[1].Stress != 0 || morae[4].Consonant != "t" || morae[4].Vowel != "al" {
		t.Fatalf("morae=%+v", morae)
	}
	if morae[1].WordIndex != 0 || !morae[1].WordEnd || morae[3].WordIndex != 1 {
		t.Fatalf("word positions=%+v", morae)
	}
	// 手で直した読みはそのまま使う。
	if _, edited, err := ParseSpanish("Hola", "ˈo.lˌa", nil); err != nil || len(edited) != 2 || edited[1].Stress != 2 {
		t.Fatalf("edited=%+v err=%v", edited, err)
	}
}

func TestSpanishNumberWords(t *testing.T) {
	cases := map[string]string{
		"0": "cero", "21": "veintiuno", "35": "treinta y cinco", "100": "cien", "101": "ciento uno",
		"500": "quinientos", "1000": "mil", "2024": "dos mil veinticuatro", "21000": "veintiún mil",
		"1000000": "un millón", "3500000": "tres millones quinientos mil",
	}
	for digits, want := range cases {
		if got := spanishNumberWords(digits); got != want {
			t.Errorf("%s: got %q, want %q", digits, got, want)
		}
	}
}
