package dataset

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/oto"
	"utautts/internal/voicebank"
)

func TestBuildConnectionsCreatesNaturalAndReplacedPairs(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.wav")
	second := filepath.Join(root, "second.wav")
	writeTone(t, first, 200)
	writeTone(t, second, 300)
	entry := func(source, alias string, offset float64, line int) oto.Entry {
		return oto.Entry{
			Filename: source, Alias: alias, Offset: offset, Fixed: 80,
			Preutterance: 40, Overlap: 15, OtoPath: filepath.Join(root, "oto.ini"), Line: line,
		}
	}
	bank := &voicebank.Bank{Root: root, Name: "test", Entries: map[string][]oto.Entry{
		"- あ": {entry(first, "- あ", 50, 1)},
		"a い": {entry(first, "a い", 200, 2)},
		"- う": {entry(second, "- う", 50, 3)},
		"u い": {entry(second, "u い", 200, 4)},
	}}

	records, report := BuildConnections(bank, ConnectionConfig{NegativesPerPositive: 1})
	if report.PositiveRecords != 2 || report.NegativeRecords != 2 || len(records) != 4 {
		t.Fatalf("report=%+v records=%d", report, len(records))
	}
	for index := 0; index < len(records); index += 2 {
		positive, negative := records[index], records[index+1]
		if positive.Label != 1 || positive.Provenance != "natural_continuation" || positive.Previous.Source != positive.Current.Source {
			t.Fatalf("positive=%+v", positive)
		}
		if negative.Label != 0 || negative.Provenance != "replaced_right" || negative.Previous.Source == negative.Current.Source {
			t.Fatalf("negative=%+v", negative)
		}
		if positive.GroupID != negative.GroupID || positive.Current.AliasKey != negative.Current.AliasKey {
			t.Fatalf("pair grouping mismatch: positive=%+v negative=%+v", positive, negative)
		}
	}
}

func TestHardNegativePrefersPlausibleAcousticJoin(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "a-first.wav")
	closeMatch := filepath.Join(root, "b-close.wav")
	farMatch := filepath.Join(root, "c-far.wav")
	writeTone(t, first, 200)
	writeTone(t, closeMatch, 205)
	writeTone(t, farMatch, 500)
	entry := func(source, alias string, offset float64, line int) oto.Entry {
		return oto.Entry{Filename: source, Alias: alias, Offset: offset, Fixed: 80,
			Preutterance: 40, Overlap: 15, OtoPath: filepath.Join(root, "oto.ini"), Line: line}
	}
	bank := &voicebank.Bank{Root: root, Name: "test", Entries: map[string][]oto.Entry{
		"- あ": {
			entry(first, "- あ", 50, 1), entry(closeMatch, "- あ", 50, 3), entry(farMatch, "- あ", 50, 5),
		},
		"a い": {
			entry(first, "a い", 200, 2), entry(closeMatch, "a い", 200, 4), entry(farMatch, "a い", 200, 6),
		},
	}}
	records, _ := BuildConnections(bank, ConnectionConfig{NegativesPerPositive: 1, Limit: 1, NegativeStrategy: "hard"})
	if len(records) != 2 {
		t.Fatalf("records=%d", len(records))
	}
	if records[1].Current.Source != filepath.Base(closeMatch) {
		t.Fatalf("hard negative source=%q, want close match", records[1].Current.Source)
	}
}

func writeTone(t *testing.T, path string, frequency float64) {
	t.Helper()
	const sampleRate = 16000
	data := make([]int16, sampleRate/2)
	for index := range data {
		data[index] = int16(8000 * math.Sin(2*math.Pi*frequency*float64(index)/sampleRate))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
}
