package ipapron

import (
	"bytes"
	"testing"

	"github.com/ikawaha/kagome-dict/ipa"
)

func TestEmbeddedTableMatchesEveryFullDictionaryPronunciation(t *testing.T) {
	full := ipa.Dict()
	built, err := Build(full)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(built, data) {
		t.Fatal("pron.bin is stale; run go generate ./internal/frontend/ipapron")
	}
	table, err := Load(full)
	if err != nil {
		t.Fatal(err)
	}
	column := int(full.ContentsMeta["_pronunciation"])
	for id, row := range full.Contents {
		want := row[column-len(full.POSTable.POSs[id])]
		if got, ok := table.Pronunciation(id); !ok || got != want {
			t.Fatalf("morph %d: got %q want %q", id, got, want)
		}
	}
	if _, ok := table.Pronunciation(len(full.Morphs)); ok {
		t.Fatal("out-of-range morph returned a pronunciation")
	}
	if _, ok := table.Pronunciation(-1); ok {
		t.Fatal("negative morph returned a pronunciation")
	}
}

func TestShrinkDictionaryIsAcceptedAndOtherDictionariesAreRejected(t *testing.T) {
	shrink := ipa.DictShrink()
	if _, err := Load(shrink); err != nil {
		t.Fatalf("shrink dictionary rejected: %v", err)
	}
	if _, err := parse(data, DictHash(shrink)+1, len(shrink.Morphs)); err == nil {
		t.Fatal("different dictionary hash accepted")
	}
	if _, err := parse(data, DictHash(shrink), len(shrink.Morphs)+1); err == nil {
		t.Fatal("different morph count accepted")
	}
}

func TestCorruptTablesAreRejected(t *testing.T) {
	shrink := ipa.DictShrink()
	hash, count := DictHash(shrink), len(shrink.Morphs)
	for name, corrupt := range map[string][]byte{
		"empty":     nil,
		"magic":     append([]byte("XXXXXXXX"), data[8:]...),
		"truncated": data[:len(data)/2],
	} {
		if _, err := parse(corrupt, hash, count); err == nil {
			t.Fatalf("%s table accepted", name)
		}
	}
	badCode := bytes.Clone(data)
	badCode[len(badCode)-1] = 0xff
	if _, err := parse(badCode, hash, count); err == nil {
		t.Fatal("unknown character code accepted")
	}
}
