package es

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
	got, err := Parse("Hola, ¿qué tal?", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reading != "ˈo.la , kˈe tˈal ?" {
		t.Fatalf("reading=%q", got.Reading)
	}
	var texts []string
	for _, mora := range got.Morae {
		texts = append(texts, mora.Text)
	}
	// 文中の読点は休止のモーラになり、文末の記号はモーラにしない。
	if joined := strings.Join(texts, " "); joined != "o la , ke tal" {
		t.Fatalf("morae=%q", joined)
	}
	if !got.Morae[2].Pause || got.Morae[0].Stress != 1 || got.Morae[1].Stress != 0 || got.Morae[4].Consonant != "t" || got.Morae[4].Vowel != "al" {
		t.Fatalf("morae=%+v", got.Morae)
	}
	if got.Morae[1].WordIndex != 0 || !got.Morae[1].WordEnd || got.Morae[3].WordIndex != 1 {
		t.Fatalf("word positions=%+v", got.Morae)
	}
	// 手で直した読みはそのまま使う。
	if edited, err := Parse("Hola", "ˈo.lˌa", nil); err != nil || len(edited.Morae) != 2 || edited.Morae[1].Stress != 2 {
		t.Fatalf("edited=%+v err=%v", edited, err)
	}
}

func TestSpanishCurrencyAndPercent(t *testing.T) {
	cases := map[string]string{
		"El precio es 3,50 €.": "ˈeʊ.ɾos kon",
		"El descuento es 50%.": "poɾ θjˈɛn.to",
		"Cuesta US$ 3,50.":     "ðˈo.la.ɾes kon",
		"Son 3,50 metros.":     "kˈo.ma",
	}
	for text, want := range cases {
		got, err := Parse(text, "", nil)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if !strings.Contains(got.Reading, want) {
			t.Errorf("%q: reading %q does not contain %q", text, got.Reading, want)
		}
	}
}

func TestSpanishNumberWords(t *testing.T) {
	tests := map[string]string{
		"0":       "cero",
		"21":      "veintiuno",
		"35":      "treinta y cinco",
		"100":     "cien",
		"101":     "ciento uno",
		"500":     "quinientos",
		"1000":    "mil",
		"2024":    "dos mil veinticuatro",
		"21000":   "veintiún mil",
		"1000000": "un millón",
		"3500000": "tres millones quinientos mil",
	}
	for input, want := range tests {
		if got := SpanishNumberWords(input); got != want {
			t.Errorf("SpanishNumberWords(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSpanishForeignNamesUseDictionary(t *testing.T) {
	cases := map[string]string{
		"Google y ChatGPT.":   "ˈɡu.ɣel i ʧˈat.xe.pe.tˈe .",
		"Microsoft y OpenAI.": "mˌi.kɾo.sˈoft i ˌo.pen.ˈaj .",
	}
	for text, want := range cases {
		got, err := Parse(text, "", nil)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if got.Reading != want {
			t.Errorf("%q: got %q, want %q", text, got.Reading, want)
		}
	}
}
