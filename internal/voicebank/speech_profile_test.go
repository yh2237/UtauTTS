package voicebank

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

func TestSpeechProfileBoundedAndInvalidated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.wav")
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
	for i := range pcm.Data {
		pcm.Data[i] = int16(8000 * math.Sin(2*math.Pi*200*float64(i)/16000))
	}
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	entry := oto.Entry{Filename: path, Preutterance: 60, Fixed: 100}
	bank := &Bank{}
	profile := bank.CalibrateSpeech(entry)
	if !profile.Applied || math.Abs(profile.SuggestedFixedMS-100) > 20 || math.Abs(profile.F0Hz-200) > 5 {
		t.Fatalf("profile: %+v", profile)
	}
	for i := range pcm.Data {
		pcm.Data[i] = 0
	}
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(time.Second)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	silent := bank.CalibrateSpeech(entry)
	if silent.Applied || silent.SuggestedFixedMS != 100 || silent.Reason != "no-stable-voicing" {
		t.Fatalf("stale profile: %+v", silent)
	}
	bank.ClearSpeechProfiles()
	if len(bank.speechProfiles) != 0 {
		t.Fatal("clear failed")
	}
}

func TestCoverageReportsRequiredCodaButNotOptionalRelease(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{"a": {{Filename: "a.wav", Alias: "a"}}, "s t": {{Filename: "st.wav", Alias: "s t"}}}}
	mora := frontend.Mora{Text: "a", Vowel: "a", Aliases: &frontend.AliasHints{Main: []string{"a"}, Endings: [][]string{{"a s"}, {"s t"}, {"a -"}}, EndingPhones: [][]string{{"s"}, {"t"}, nil}}}
	selected, err := bank.Resolve([]frontend.Mora{mora})
	if err != nil {
		t.Fatal(err)
	}
	if len(selected[0].MissingPhones) != 1 || selected[0].MissingPhones[0].Phones[0] != "s" || selected[0].Endings[0].EndingIndex != 1 {
		t.Fatalf("selection: %+v", selected[0])
	}
	coverage, err := bank.AuditCoverage([]frontend.Mora{mora}, "")
	if err != nil || coverage.Covered != 1 || len(coverage.MissingPhones) != 1 {
		t.Fatalf("coverage: %+v %v", coverage, err)
	}
}

func TestInitialClusterFallbackReportsMissingPhone(t *testing.T) {
	_, morae, err := frontend.ParseEnglishDelta("", "S T AA1", nil)
	if err != nil {
		t.Fatal(err)
	}
	bank := &Bank{Entries: map[string][]oto.Entry{"tA": {{Alias: "tA", Filename: "ta.wav"}}}}
	coverage, err := bank.AuditCoverage(morae, "")
	if err != nil || len(coverage.MissingPhones) != 1 || coverage.MissingPhones[0].Phones[0] != "s" {
		t.Fatalf("coverage %+v %v", coverage, err)
	}
	bank.Entries["st"] = []oto.Entry{{Alias: "st", Filename: "st.wav"}}
	coverage, err = bank.AuditCoverage(morae, "")
	if err != nil || len(coverage.MissingPhones) != 0 {
		t.Fatalf("cluster bridge ignored %+v %v", coverage, err)
	}
}

func TestEnglishCodaBeforeClusterUsesAvailableRecording(t *testing.T) {
	_, morae, err := frontend.ParseEnglishDelta("", "N EH1 K S T | S T EH1 P S | SP", nil)
	if err != nil {
		t.Fatal(err)
	}
	bank := &Bank{Entries: map[string][]oto.Entry{}}
	for _, alias := range []string{"nE", "stE", "E k", "k st-", "E p", "p s-"} {
		bank.Entries[alias] = []oto.Entry{{Alias: alias, Filename: "source.wav"}}
	}
	selected, err := bank.Resolve(morae)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected[0].MissingPhones) != 0 || len(selected[0].Endings) != 2 || selected[0].Endings[1].Alias != "k st-" {
		t.Fatalf("lost recorded coda: %+v", selected[0])
	}
	delete(bank.Entries, "k st-")
	selected, err = bank.Resolve(morae)
	if err != nil || len(selected[0].MissingPhones) != 1 {
		t.Fatal("missing coda no longer reported", selected, err)
	}
	if got := selected[0].MissingPhones[0].Phones; len(got) != 2 || got[0] != "s" || got[1] != "t" {
		t.Fatal("wrong missing phones", got)
	}
}
