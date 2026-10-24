//go:build !js

package tts

import (
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

func TestAlignEnglishWordFollowsPhones(t *testing.T) {
	phones := func(symbols ...string) []frontend.Phone {
		out := make([]frontend.Phone, len(symbols))
		for i, s := range symbols {
			out[i] = frontend.Phone{Symbol: s}
		}
		return out
	}
	for _, test := range []struct {
		symbols []string
		phones  []frontend.Phone
		want    []int
	}{
		// hello: həlˈO
		{[]string{"h", "ə", "l", "ˈ", "O"}, phones("hh", "ah", "l", "ow"), []int{0, 1, 2, 3, 3}},
		// weather: wˈɛðəɹ
		{[]string{"w", "ˈ", "ɛ", "ð", "ə", "ɹ"}, phones("w", "eh", "dh", "er"), []int{0, 1, 1, 2, 3, 3}},
		// today: tədˈA
		{[]string{"t", "ə", "d", "ˈ", "A"}, phones("t", "ah", "d", "ey"), []int{0, 1, 2, 3, 3}},
		// really: ɹˈiᵊli
		{[]string{"ɹ", "ˈ", "i", "ᵊ", "l", "i"}, phones("r", "ih", "l", "iy"), []int{0, 1, 1, 1, 2, 3}},
	} {
		if got := alignEnglishWord(test.symbols, test.phones); !reflect.DeepEqual(got, test.want) {
			t.Errorf("alignEnglishWord(%q) = %v, want %v", test.symbols, got, test.want)
		}
	}
}
