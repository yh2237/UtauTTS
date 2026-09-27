package worldline

import (
	"os"
	"path/filepath"
	"testing"
	"utautts/internal/plan"
	"utautts/internal/render/base"
	"utautts/internal/voicebank"
)

func TestSourceLibraryIntegrationRejectsNilPlan(t *testing.T) {
	if _, err := renderWorldlineEngine(nil, base.Config{}, "utautts-world-phrase"); err == nil {
		t.Fatal("nil plan accepted")
	}
}

func TestSourceLibraryUsesCurrentPhoneTimingAndRejectsRepeatedCoda(t *testing.T) {
	p := &plan.Plan{Language: "en", Units: []plan.Unit{{Alias: "e k", Role: "ending", Position: 7, NoteStartMS: 840, DurationMS: 70, CodaPhones: []string{"k"}}}, PhoneTimings: []plan.PhoneTiming{{Position: 7, Symbol: "eh", Role: "nucleus", StartMS: 700, DurationMS: 140}, {Position: 7, Symbol: "k", Role: "coda", StartMS: 840, DurationMS: 70}}}
	record := voicebank.SourcePhoneRecord{SourceSHA256: "hash", Phones: []voicebank.SourcePhoneInterval{{Symbol: "eh", StartMS: 0, EndMS: 210}, {Symbol: "k", StartMS: 210, EndMS: 340}, {Symbol: "eh", StartMS: 340, EndMS: 530}}}
	span, ok := sourceRecordSpan(p, 0, record, voicebank.SourceAnalysis{})
	if !ok || span.CoreEndMS != 340 || span.Mappings[0].RequestedStartMS != 840 {
		t.Fatal(span, ok)
	}
	record.Phones = append(record.Phones, voicebank.SourcePhoneInterval{Symbol: "k", StartMS: 530, EndMS: 550})
	if _, ok = sourceRecordSpan(p, 0, record, voicebank.SourceAnalysis{}); ok {
		t.Fatal("ambiguous coda applied")
	}
}

func TestNormalSourceLibraryCanBeDisabled(t *testing.T) {
	disabled := false
	dir := t.TempDir()
	data := `{"version":1,"language":"en","time_origin":"oto-offset","entries":[{"source_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","duration_ms":100,"phones":[{"symbol":"d","start_ms":0,"end_ms":100}]}]}`
	if err := os.WriteFile(filepath.Join(dir, "source-phone-library.json"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{Language: "en", Voicebank: dir, PhoneTimingSource: "multilingual-speech-score-v1"}
	if len(sourceLibraries(p, base.Config{})) == 0 {
		t.Fatal("default mapping did not load bank-local library")
	}
	if libraries := sourceLibraries(p, base.Config{ProviderOptions: base.ProviderOptions{Worldline: base.WorldlineProviderOptions{SourcePhoneMapping: &disabled}}}); len(libraries) != 0 {
		t.Fatal("disabled library still loaded")
	}
}

func TestChineseSourceLibraryMapsEntireNasalSyllable(t *testing.T) {
	p := &plan.Plan{Language: "zh", Units: []plan.Unit{{Alias: "hen", Role: "mora", Position: 2, NoteStartMS: 500, DurationMS: 200, CodaPhones: []string{"n"}}}, PhoneTimings: []plan.PhoneTiming{{Position: 2, Symbol: "h", Role: "onset", StartMS: 500, DurationMS: 40}, {Position: 2, Symbol: "e", Role: "nucleus", StartMS: 540, DurationMS: 110}, {Position: 2, Symbol: "n", Role: "coda", StartMS: 650, DurationMS: 50}}}
	r := voicebank.SourcePhoneRecord{Phones: []voicebank.SourcePhoneInterval{{Symbol: "h", StartMS: 0, EndMS: 40}, {Symbol: "e", StartMS: 40, EndMS: 200}, {Symbol: "n", StartMS: 200, EndMS: 300}}}
	span, ok := sourceRecordSpan(p, 0, r, voicebank.SourceAnalysis{})
	if !ok || len(span.Mappings) != 3 || span.CoreStartMS != 0 || span.Mappings[2].RequestedStartMS != 650 {
		t.Fatal(span, ok)
	}
	span, ok = sourceRecordSpan(p, 0, voicebank.SourcePhoneRecord{Phones: []voicebank.SourcePhoneInterval{{Symbol: "e", StartMS: 0, EndMS: 200}, {Symbol: "n", StartMS: 200, EndMS: 300}}}, voicebank.SourceAnalysis{})
	if ok {
		t.Fatal("incomplete source syllable applied", span)
	}
}
