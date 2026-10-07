package openjtalk

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestMoraPhonesForMFA(t *testing.T) {
	// 期待値はpyopenjtalk 0.4.1のg2pをMFA音素へ変換した結果。
	cases := map[string]string{"か": "k a", "き": "c i", "し": "ɕ i", "じ": "dʑ i", "ち": "tɕ i", "つ": "ts ɯ", "ふ": "ɸ ɯ", "ぎ": "ɟ i", "きゃ": "c a", "てぃ": "t i", "っ": "ʔ", "ん": "ɴ", "あ": "a", "かん": "k a ɴ"}
	for mora, want := range cases {
		if got := strings.Join(MoraPhones(mora), " "); got != want {
			t.Errorf("%s: got %q, want %q", mora, got, want)
		}
	}
}
func TestCapturedMFAMoraPhones(t *testing.T) {
	data, err := os.ReadFile("testdata/mfa_mora_phones.json")
	if err != nil {
		t.Fatal(err)
	}
	var expected map[string][]string
	if err := json.Unmarshal(data, &expected); err != nil {
		t.Fatal(err)
	}
	for mora, want := range expected {
		if got := MoraPhones(mora); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %v, want %v", mora, got, want)
		}
	}
}
