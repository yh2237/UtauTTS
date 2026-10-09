package voicebank

import (
	"reflect"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

func TestVCCVUsesRecordedNasalAndSPCluster(t *testing.T) {
	for _, tc := range []struct {
		reading        string
		aliases, codas []string
	}{
		{"K R IH1 S P", []string{"kri", "i sp"}, []string{"s", "p"}},
		{"K R IH1 S P", []string{"kri", "i s", "s p-"}, []string{"s", "p"}},
		{"D R IH1 NG K S", []string{"dri", "1ng", "ng k", "k s-"}, []string{"ng", "k", "s"}},
	} {
		_, morae, err := frontend.ParseEnglishVCCV("", tc.reading, nil)
		if err != nil {
			t.Fatal(err)
		}
		bank := &Bank{Entries: map[string][]oto.Entry{}}
		for _, alias := range tc.aliases {
			bank.Entries[alias] = []oto.Entry{{Alias: alias, Filename: "fixture.wav"}}
		}
		selected, err := bank.Resolve(morae)
		if err != nil || len(selected[0].MissingPhones) != 0 {
			t.Fatalf("%s: %v, %v", tc.reading, selected, err)
		}
		var got []string
		for _, unit := range selected[0].Endings {
			got = append(got, unit.CodaPhones...)
		}
		if !reflect.DeepEqual(got, tc.codas) {
			t.Fatalf("coda omitted or duplicated: %v, want %v", got, tc.codas)
		}
	}
}

func TestDeltaVelarNasalConnectionKeepsAllPhones(t *testing.T) {
	_, morae, err := frontend.ParseEnglishDelta("", "D R IH1 NG K S", nil)
	if err != nil {
		t.Fatal(err)
	}
	bank := &Bank{Entries: map[string][]oto.Entry{}}
	for _, alias := range []string{"drI", "I N", "n ks-"} {
		bank.Entries[alias] = []oto.Entry{{Alias: alias, Filename: "fixture.wav"}}
	}
	selected, err := bank.Resolve(morae)
	if err != nil || len(selected[0].MissingPhones) != 0 || len(selected[0].Endings) != 2 {
		t.Fatalf("velar coda coverage: %v, %v", selected, err)
	}
	if !reflect.DeepEqual(selected[0].Endings[1].CodaPhones, []string{"k", "s"}) {
		t.Fatal(selected[0].Endings)
	}
}

func TestEnglishMissingCompoundCodaUsesRecordedChain(t *testing.T) {
	_, m, err := frontend.ParseEnglishDelta("", "T EH1 K S T S", nil)
	if err != nil {
		t.Fatal(err)
	}
	b := &Bank{Entries: map[string][]oto.Entry{}}
	for _, alias := range []string{"tE", "E k", "k s", "s t", "t s-"} {
		b.Entries[alias] = []oto.Entry{{Alias: alias, Filename: "source.wav"}}
	}
	selected, err := b.Resolve(m)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for i, e := range selected[0].Endings {
		got = append(got, e.Alias)
		if e.CodaStart != i || len(e.CodaPhones) != 1 {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(got, []string{"E k", "k s", "s t", "t s-"}) || len(selected[0].MissingPhones) != 0 {
		t.Fatal(got, selected[0].MissingPhones)
	}
	b.Entries["k sts-"] = []oto.Entry{{Alias: "k sts-", Filename: "compound.wav"}}
	selected, err = b.Resolve(m)
	if err != nil || len(selected[0].Endings) != 2 || len(selected[0].Endings[1].CodaPhones) != 3 {
		t.Fatal(selected, err)
	}
	delete(b.Entries, "k sts-")
	b.Entries["k st"] = []oto.Entry{{Alias: "k st", Filename: "two-phones.wav"}}
	selected, err = b.Resolve(m)
	if err != nil || len(selected[0].Endings) != 3 || !reflect.DeepEqual(selected[0].Endings[1].CodaPhones, []string{"s", "t"}) || selected[0].Endings[2].CodaStart != 3 {
		t.Fatal("did not select shorter fully covered path", selected, err)
	}
	delete(b.Entries, "k st")
	delete(b.Entries, "s t")
	selected, err = b.Resolve(m)
	if err != nil || len(selected[0].MissingPhones) != 1 || !reflect.DeepEqual(selected[0].MissingPhones[0].Phones, []string{"t"}) || selected[0].Endings[2].CodaStart != 3 {
		t.Fatal(selected, err)
	}
}

func TestVCCVCodaChainUsesToneAffixes(t *testing.T) {
	_, m, err := frontend.ParseEnglishVCCV("", "T EH1 K S T S", nil)
	if err != nil {
		t.Fatal(err)
	}
	b := &Bank{Entries: map[string][]oto.Entry{}, PrefixMap: map[string]Affix{"C4": {Suffix: "B3"}}}
	for _, alias := range []string{"teB3", "ekB3", "kstB3", "ts-B3"} {
		b.Entries[alias] = []oto.Entry{{Alias: alias, Filename: "source.wav"}}
	}
	selected, err := b.Resolve(m)
	if err != nil || len(selected[0].MissingPhones) != 0 || len(selected[0].Endings) != 3 {
		t.Fatal(selected, err)
	}
	if selected[0].Endings[1].Alias != "kstB3" || len(selected[0].Endings[1].CodaPhones) != 2 {
		t.Fatal(selected)
	}
}

func TestCodaAlternativeSegmentationUsesJoinScore(t *testing.T) {
	_, moras, err := frontend.ParseEnglishDelta("", "T EH1 K S T S", nil)
	if err != nil {
		t.Fatal(err)
	}
	bank := &Bank{Entries: map[string][]oto.Entry{}}
	for _, alias := range []string{"tE", "E k", "k st", "k s", "s t", "t s-"} {
		group := "A"
		if alias == "k st" {
			group = "B"
		}
		bank.Entries[alias] = []oto.Entry{{Alias: alias, Filename: "fixture.wav", SourceGroup: group}}
	}
	selected, err := bank.Resolve(moras)
	if err != nil {
		t.Fatal(err)
	}
	var aliases []string
	for _, ending := range selected[0].Endings {
		aliases = append(aliases, ending.Alias)
	}
	if !reflect.DeepEqual(aliases, []string{"E k", "k s", "s t", "t s-"}) || len(selected[0].MissingPhones) != 0 {
		t.Fatalf("join score did not choose covered alternative: %v, %v", aliases, selected[0].MissingPhones)
	}
}
