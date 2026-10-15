package plan

import (
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/oto"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func TestOtoCodaSpans(t *testing.T) {
	mora := frontend.Mora{Language: frontend.LanguageEnglish, Phones: []frontend.Phone{
		{Symbol: "hh", Role: "onset"}, {Symbol: "eh", Role: "nucleus"}, {Symbol: "l", Role: "coda"}, {Symbol: "p", Role: "coda"},
	}}
	endings := []voicebank.Selection{
		{CodaPhones: []string{"l"}, CodaStart: 0, Entry: oto.Entry{Preutterance: 120}},
		{CodaPhones: []string{"p"}, CodaStart: 1, Entry: oto.Entry{Preutterance: 100}},
	}
	near := func(got, want []float64) bool {
		for i := range want {
			if math.Abs(got[i]-want[i]) > 1e-9 {
				return false
			}
		}
		return true
	}
	if got := otoCodaSpans(mora, []float64{40, 200, 50, 50}, endings); !near(got, []float64{40, 138, 62, 100}) {
		t.Fatalf("spans = %v", got)
	}
	scale := 105.0 / 162
	if got := otoCodaSpans(mora, []float64{40, 50, 50, 50}, endings); !near(got, []float64{40, 45, 62 * scale, 100 * scale}) {
		t.Fatalf("limited spans = %v", got)
	}
	if got := otoCodaSpans(mora, []float64{40, 200, 50, 50}, nil); !near(got, []float64{40, 200, 50, 50}) {
		t.Fatalf("spans without endings = %v", got)
	}
}
