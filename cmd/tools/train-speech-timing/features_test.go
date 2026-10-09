package main

import (
	"reflect"
	"testing"
)

func TestVocabularyFromSymbols(t *testing.T) {
	vocab := vocabularyFromSymbols([]string{"ow", "sil", "ah", "ow", "", " "})
	want := []string{"<pad>", "<unk>", "sil", "ah", "ow"}
	if !reflect.DeepEqual(vocab.names, want) {
		t.Fatalf("names = %v, want %v", vocab.names, want)
	}
	if vocab.silence != 2 || vocab.unknown != 1 || vocab.id("ow") != 4 || vocab.id("missing") != 1 {
		t.Fatalf("ids: silence=%d unknown=%d ow=%d missing=%d", vocab.silence, vocab.unknown, vocab.id("ow"), vocab.id("missing"))
	}
}

func TestPhonesFromTokensFillsGaps(t *testing.T) {
	tokens := []corpusToken{
		{StartMS: 50, EndMS: 100, Phones: []corpusPhone{{Symbol: "hh", StartMS: 50, EndMS: 60}, {Symbol: "ah", StartMS: 60, EndMS: 100}}},
		{Pause: true, StartMS: 100, EndMS: 150},
		{StartMS: 150, EndMS: 200, Phones: []corpusPhone{{Symbol: "l", StartMS: 150, EndMS: 170}, {Symbol: "ow", StartMS: 170, EndMS: 200}}},
	}
	phones := phonesFromTokens(tokens)
	var labels []string
	for _, p := range phones {
		labels = append(labels, p.name)
	}
	want := []string{"sil", "hh", "ah", "sil", "l", "ow"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %v, want %v", labels, want)
	}
	if phones[0].start != 0 || phones[0].end != 0.05 {
		t.Fatalf("leading sil = %v..%v", phones[0].start, phones[0].end)
	}
}

func TestFrameInputsUsesVocabulary(t *testing.T) {
	vocab := vocabularyFromSymbols([]string{"ah", "ow"})
	f0 := make([]float64, 10)
	ps := []phone{{0, 0.05, "ah", "ah"}, {0.05, 0.1, "ow", "ow"}}
	ids, _ := frameInputs(ps, f0, vocab)
	if ids[0] != vocab.id("ah") || ids[15] != vocab.id("ow") {
		t.Fatalf("ids = %d/%d, want %d/%d", ids[0], ids[15], vocab.id("ah"), vocab.id("ow"))
	}
	empty := make([]float64, 3)
	defaults, _ := frameInputs(nil, empty, vocab)
	if defaults[0] != vocab.silence {
		t.Fatalf("default id = %d, want silence %d", defaults[0], vocab.silence)
	}
}
