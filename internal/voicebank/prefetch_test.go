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
