package main

import (
	"math"
	"testing"

	"utautts/cmd/tools/internal/aishell3"
)

const sampleGrid = `File type = "ooTextFile"
Object class = "TextGrid"

xmin = 0 
xmax = 1.2 
tiers? <exists> 
size = 2 
item []: 
    item [1]:
        class = "IntervalTier" 
        name = "words" 
        xmin = 0 
        xmax = 1.2 
        intervals: size = 3 
        intervals [1]:
            xmin = 0 
            xmax = 0.1 
            text = "" 
        intervals [2]:
            xmin = 0.1 
            xmax = 0.5 
            text = "guang3" 
        intervals [3]:
            xmin = 0.5 
            xmax = 0.9 
            text = "zhou1" 
    item [2]:
        class = "IntervalTier" 
        name = "phones" 
        xmin = 0 
        xmax = 1.2 
        intervals: size = 5 
        intervals [1]:
            xmin = 0 
            xmax = 0.1 
            text = "" 
        intervals [2]:
            xmin = 0.1 
            xmax = 0.16 
            text = "g" 
        intervals [3]:
            xmin = 0.16 
            xmax = 0.5 
            text = "uang3" 
        intervals [4]:
            xmin = 0.5 
            xmax = 0.58 
            text = "zh" 
        intervals [5]:
            xmin = 0.58 
            xmax = 0.9 
            text = "ou1" 
`

func TestTierIntervalsSkipsEmptyText(t *testing.T) {
	words := aishell3.TextGridIntervals(sampleGrid, "words")
	phones := aishell3.TextGridIntervals(sampleGrid, "phones")
	if len(words) != 2 || len(phones) != 4 {
		t.Fatalf("words=%d phones=%d, want 2/4", len(words), len(phones))
	}
	if words[0].Text != "guang3" || phones[0].Text != "g" {
		t.Fatalf("intervals = %+v / %+v", words[0], phones[0])
	}
}

func TestBuildTokensUsesMeasuredBoundaries(t *testing.T) {
	tokens, err := buildTokens(aishell3.TextGridIntervals(sampleGrid, "words"), aishell3.TextGridIntervals(sampleGrid, "phones"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 2 {
		t.Fatalf("tokens = %d, want 2", len(tokens))
	}
	first := tokens[0]
	if len(first.Phones) != 4 || first.Phones[0].Symbol != "g" || first.Phones[0].Role != "onset" {
		t.Fatalf("first syllable phones = %+v", first.Phones)
	}
	if math.Abs(first.Phones[0].StartMS-100) > 1 || math.Abs(first.Phones[0].EndMS-160) > 1 {
		t.Fatalf("initial span = %v..%v, want 100..160", first.Phones[0].StartMS, first.Phones[0].EndMS)
	}
	last := first.Phones[len(first.Phones)-1]
	if last.Symbol != "ng" || math.Abs(last.EndMS-500) > 1 {
		t.Fatalf("final coda = %+v, want ng ending at 500", last)
	}
	second := tokens[1]
	if len(second.Phones) != 3 || second.Phones[0].Symbol != "zh" {
		t.Fatalf("second syllable phones = %+v", second.Phones)
	}
	if math.Abs(second.Phones[0].StartMS-500) > 1 || math.Abs(second.Phones[0].EndMS-580) > 1 {
		t.Fatalf("second initial span = %v..%v, want 500..580", second.Phones[0].StartMS, second.Phones[0].EndMS)
	}
}
