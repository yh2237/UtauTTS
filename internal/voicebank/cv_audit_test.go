package voicebank

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/oto"
)

func TestAuditSingleCVReportsSourceTiming(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ka.wav")
	data := make([]int16, 300)
	for i := range data {
		data[i] = int16(5000 * math.Sin(2*math.Pi*180*float64(i)/1000))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 1000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	bank := &Bank{Root: dir, Entries: map[string][]oto.Entry{
		"か":   {{Filename: path, Alias: "か", Offset: 0, Fixed: 180, Preutterance: 130, Blank: 0}},
		"- か": {{Filename: path, Alias: "- か", Offset: 0, Fixed: 180, Preutterance: 130}},
		"a か": {{Filename: path, Alias: "a か", Offset: 0, Fixed: 100, Preutterance: 50}},
	}}
	audit, err := bank.AuditSingleCV()
	if err != nil {
		t.Fatal(err)
	}
	if audit.SingleCVBank || audit.CVEntries != 1 || audit.InitialContextEntries != 1 || audit.ContextVCVEntries != 1 {
		t.Fatalf("inventory = %+v", audit)
	}
	if audit.WarningEntryCount != 2 {
		t.Fatalf("warning count = %d; entries=%+v", audit.WarningEntryCount, audit.Entries)
	}
	if audit.Entries[0].TrimmedLengthMS != 300 || audit.Entries[0].VowelTailMS != 120 {
		t.Fatalf("source metrics = %+v", audit.Entries[0])
	}
	contextWarnings := []string(nil)
	for _, entry := range audit.Entries {
		if entry.ContextVCV {
			contextWarnings = entry.Warnings
		}
	}
	if audit.VCVProfiledEntries != 1 || len(contextWarnings) != 0 {
		t.Fatalf("VCV profile metrics = %+v", audit)
	}
}

func TestAuditTimingWarningsUseVCVLimits(t *testing.T) {
	entry := oto.Entry{Preutterance: 210, Fixed: 360, Overlap: 70}
	warnings := auditTimingWarnings(entry, true)
	for _, warning := range warnings {
		if warning == "long-preutterance" || warning == "long-fixed" {
			t.Fatalf("VCV timing used CV warning: %v", warnings)
		}
	}
	if len(warnings) != 1 || warnings[0] != "vcv-preutterance-clamp-needed" {
		t.Fatalf("VCV warnings = %v", warnings)
	}
}
