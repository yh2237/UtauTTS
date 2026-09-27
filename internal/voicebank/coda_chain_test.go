package voicebank

import (
	"reflect"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

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
