package reference

import (
	"reflect"
	"testing"

	"github.com/yh2237/UtauIDX/speechpool"
	"github.com/yh2237/UtauTTS/internal/oto"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func TestReferenceRecordingsCarryRawRegions(t *testing.T) {
	bank := &voicebank.Bank{Entries: map[string][]oto.Entry{
		"a":  {{Filename: "a.wav", Alias: "a", Offset: 20, Blank: -80}},
		"ka": {{Filename: "a.wav", Alias: "ka", Offset: 120, Blank: 40}},
		"i":  {{Filename: "i.wav", Alias: "i", Offset: 30, Blank: -100}},
	}}
	want := []speechpool.Entry{
		{Path: "a.wav", Layer: speechpool.LayerRaw, Regions: []speechpool.Region{{Offset: 20, Blank: -80}, {Offset: 120, Blank: 40}}},
		{Path: "i.wav", Layer: speechpool.LayerRaw, Regions: []speechpool.Region{{Offset: 30, Blank: -100}}},
	}
	got := referenceRecordings(bank, "", "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recordings = %+v, want %+v", got, want)
	}
}

func TestReferenceRecordingsFollowToneViaPrefixMap(t *testing.T) {
	bank := &voicebank.Bank{
		Entries: map[string][]oto.Entry{
			"あ_B3":  {{Filename: "B3/a.wav", Alias: "あ_B3", Offset: 10, Blank: -50}},
			"あ_D#4": {{Filename: "D#4/a.wav", Alias: "あ_D#4", Offset: 15, Blank: -60}},
		},
		PrefixMap: map[string]voicebank.Affix{
			"C4":  {Suffix: "_B3"},
			"D#4": {Suffix: "_D#4"},
		},
	}
	if got := referenceRecordings(bank, "", ""); len(got) != 1 || got[0].Path != "B3/a.wav" {
		t.Fatalf("default tone recordings = %+v", got)
	}
	if got := referenceRecordings(bank, "D#4", ""); len(got) != 1 || got[0].Path != "D#4/a.wav" {
		t.Fatalf("D#4 recordings = %+v", got)
	}
}
