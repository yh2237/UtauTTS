package worldline

import (
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"os"
	"path/filepath"
	"testing"
)

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
