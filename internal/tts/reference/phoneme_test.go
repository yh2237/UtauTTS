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
	got := referenceRecordings(bank, "")
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("recordings = %+v, want %+v", got, want)
	}
}
