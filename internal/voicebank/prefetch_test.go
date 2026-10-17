package voicebank

import (
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/oto"
)

func TestCandidateSourcePathsCollectsTransitionsAndFallbacksWithoutReadingWAV(t *testing.T) {
	bank := &Bank{Root: "missing", Entries: map[string][]oto.Entry{
		"あ": {{Filename: "a.wav"}}, "か": {{Filename: "ka.wav"}}, "a か": {{Filename: "aka.wav"}},
		"a k": {{Filename: "ak.wav"}}, "別": {{Filename: "unused.wav"}},
	}}
	morae, err := frontend.ParseKana("あか")
	if err != nil {
		t.Fatal(err)
	}
	got := bank.CandidateSourcePaths(morae, ResolveConfig{})
	if !reflect.DeepEqual(got, []string{"a.wav", "ak.wav", "aka.wav", "ka.wav"}) {
		t.Fatalf("paths=%v", got)
	}
	bank.Entries["尾"] = []oto.Entry{{Filename: "tail.wav"}}
	bank.Entries["代替"] = []oto.Entry{{Filename: "fallback.wav"}}
	morae[1].Aliases = &frontend.AliasHints{Main: []string{"か"}, Transition: []string{"a k"}, Endings: [][]string{{"尾"}}, EndingFallbacks: [][]frontend.CodaAlias{{{Aliases: []string{"代替"}}}}}
	got = bank.CandidateSourcePaths(morae, ResolveConfig{})
	if !reflect.DeepEqual(got, []string{"a.wav", "ak.wav", "fallback.wav", "ka.wav", "tail.wav"}) {
		t.Fatalf("hint paths=%v", got)
	}
}

func TestCandidateSourcePathsIncludesAffixedAndFallbackSources(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{"あ": {{Filename: "plain.wav"}}, "あ高": {{Filename: "high.wav"}}}, PrefixMap: map[string]Affix{"C4": {Suffix: "高"}}}
	morae, _ := frontend.ParseKana("あ、あ")
	got := bank.CandidateSourcePaths(morae, ResolveConfig{Tone: "C4"})
	if !reflect.DeepEqual(got, []string{"high.wav", "plain.wav"}) {
		t.Fatalf("paths=%v", got)
	}
}

func TestReferenceRecordingsUseDefaultPitchNormalVoice(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{
		"- aaB3": {{Filename: "B3/aa.wav"}}, "aa kB3": {{Filename: "B3/aak.wav"}},
		"- aaG4": {{Filename: "G4/aa.wav"}}, "- aaF": {{Filename: "Falsetto/aa.wav"}},
	}, Subbanks: []Subbank{
		{Suffix: "G4", ToneRanges: []ToneRange{{Low: tone(t, "G4"), High: tone(t, "B7")}}},
		{Suffix: "B3", ToneRanges: []ToneRange{{Low: tone(t, "A#3"), High: tone(t, "D4")}}},
		{Color: "Falsetto", Suffix: "F", ToneRanges: []ToneRange{{Low: tone(t, "C1"), High: tone(t, "B7")}}},
	}}
	if got := bank.ReferenceRecordings(); !reflect.DeepEqual(got, []string{"B3/aa.wav", "B3/aak.wav"}) {
		t.Fatalf("recordings=%v", got)
	}
	single := &Bank{Entries: map[string][]oto.Entry{"- aa": {{Filename: "aa.wav"}}, "aa k": {{Filename: "aak.wav"}, {Filename: "aa.wav"}}}}
	if got := single.ReferenceRecordings(); !reflect.DeepEqual(got, []string{"aa.wav", "aak.wav"}) {
		t.Fatalf("single-pitch recordings=%v", got)
	}
	// prefix.mapの接尾辞がどの別名にも付いていない音源は、すべての録音を使う。
	unmatched := &Bank{
		Entries:   map[string][]oto.Entry{"- aa": {{Filename: "aa.wav"}}, "aa k": {{Filename: "aak.wav"}}},
		PrefixMap: map[string]Affix{"C4": {Suffix: "C4"}},
	}
	if got := unmatched.ReferenceRecordings(); !reflect.DeepEqual(got, []string{"aa.wav", "aak.wav"}) {
		t.Fatalf("unmatched affix recordings=%v", got)
	}
}

func tone(t *testing.T, name string) int {
	t.Helper()
	number, ok := toneNumber(name)
	if !ok {
		t.Fatalf("bad tone %q", name)
	}
	return number
}
