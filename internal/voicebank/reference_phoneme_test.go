package voicebank

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/oto"
)

func TestReferencePhoneme(t *testing.T) {
	bank := &Bank{}
	for alias, want := range map[string]string{
		"a": "a", "あ": "a", "ア": "a", "- あ": "a", "a あ": "a", "ka": "ka", "か": "ka", "a か": "ka", "- カ": "ka",
		"ウィ": "うぃ", "おぁ": "おぁ", "あx": "", "か2": "", "か\u3099": "ga",
		"シ": "shi", "si": "shi", "ぢ": "ji", "zi": "ji", "っ": "cl", "ん": "n", "a k": "k", "a きゃ": "kya", "かB3": "ka", "": "", "息": "", "あいう": "", "a -": "",
	} {
		if got := bank.ReferencePhoneme(alias); got != want {
			t.Errorf("%q = %q, want %q", alias, got, want)
		}
	}
	bank.PrefixMap = map[string]Affix{"C4": {Prefix: "soft_", Suffix: "_C4"}}
	if got := bank.ReferencePhoneme("soft_a か_C4"); got != "ka" {
		t.Fatalf("affixed alias = %q", got)
	}
}

func TestReferenceRecordingPhoneme(t *testing.T) {
	bank := &Bank{}
	if got := bank.ReferenceRecordingPhoneme([]oto.Entry{{Alias: "あ"}, {Alias: "- あ"}, {Alias: "a"}}); got != "a" {
		t.Fatalf("equivalent aliases = %q", got)
	}
	if got := bank.ReferenceRecordingPhoneme([]oto.Entry{{Alias: "あ"}, {Alias: "か"}}); got != "" {
		t.Fatalf("mixed recording incorrectly labeled %q", got)
	}
}
