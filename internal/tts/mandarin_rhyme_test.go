package tts

import (
	"math"
	"testing"
	"utautts/internal/frontend"
)

func TestMandarinArticulationPreservesSyllableAndNasalBudget(t *testing.T) {
	for _, reading := range []string{"hao3", "tian1", "xiao3", "liu2", "gui4", "jue2"} {
		_, m, err := frontend.ParseChineseCVVC("", reading, nil)
		if err != nil {
			t.Fatal(err)
		}
		old := m[0]
		old.Phones = nil
		compound := ""
		for _, p := range m[0].Phones {
			switch p.Role {
			case "medial", "nucleus", "offglide":
				compound += p.Symbol
			case "onset":
				old.Phones = append(old.Phones, p)
			}
		}
		old.Phones = append(old.Phones, frontend.Phone{Symbol: compound, Role: "nucleus"})
		for _, p := range m[0].Phones {
			if p.Role == "coda" {
				old.Phones = append(old.Phones, p)
			}
		}
		a := speechPhoneDurations(m, 120)[0]
		b := speechPhoneDurations([]frontend.Mora{old}, 120)[0]
		sumA, sumB := 0.0, 0.0
		for _, v := range a {
			sumA += v
		}
		for _, v := range b {
			sumB += v
		}
		if math.Abs(sumA-sumB) > .001 {
			t.Fatal(reading, a, b)
		}
		if m[0].Phones[len(a)-1].Role == "coda" && a[len(a)-1] != b[len(b)-1] {
			t.Fatal("nasal slot changed", a, b)
		}
		start, length := mandarinToneWindow(m[0], a, sumA)
		if math.Abs(start-a[0]) > .001 || math.Abs(length-(sumA-a[0])) > .001 {
			t.Fatal("tone omitted voiced glide", reading, start, length)
		}
	}
}
