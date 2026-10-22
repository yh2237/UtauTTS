package frontend

import (
	"strings"
	"testing"
)

// 期待値はmisakiのEspeakG2P（espeak-ng pt-br）の出力に、音節の区切りを足したもの。
func TestPortugueseReadingMatchesKokoroPhonemes(t *testing.T) {
	cases := map[string]string{
		"obrigado":                        "ˌo.bri.ɡˈa.dʊ",
		"cidade":                          "sˌi.dˈa.ʤy",
		"pão":                             "pˈɐ̃ʊ̃",
		"gente":                           "ʒˈAŋ.ʧy",
		"Brasil":                          "bra.zˈiʊ",
		"tarde":                           "tˈaɾə.ʤy",
		"carro":                           "kˈa.xʊ",
		"exame":                           "ˌe.zˈɐ̃.my",
		"infância":                        "ˌiŋ.fˈɐ̃ŋ.sjæ",
		"arco-íris":                       "ˈaɾə.kʊ.ˈi.ɾis",
		"Você quer tomar um café comigo?": "vo.sˌe kˈɛr to.mˈaɾ ũŋ ka.fˈɛ kˌo.mˈi.ɡʊ ?",
		"O avião sai às oito horas.":      "u ˌa.vi.ˈɐ̃ʊ̃ sˈI aːz ˈoɪ.tw ˈɔ.ɾæs .",
	}
	for text, want := range cases {
		got, err := PortugueseReading(text, nil)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if got != want {
			t.Errorf("%q: got %q, want %q", text, got, want)
		}
	}
}

func TestPortugueseMoraeAreSyllables(t *testing.T) {
	_, morae, err := ParseSyllableLanguage(LanguagePortuguese, PhonemizerPortuguese, "Bom dia, tudo bem?", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, mora := range morae {
		texts = append(texts, mora.Text)
	}
	if got := strings.Join(texts, " "); got != "boŋ ʤi æ , tu dʊ bAŋ" {
		t.Fatalf("morae=%q", got)
	}
	if morae[0].Language != LanguagePortuguese || !morae[3].Pause {
		t.Fatalf("morae=%+v", morae)
	}
}
