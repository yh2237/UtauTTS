package en

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/g2p"
)

func TestHeteronymsUseContext(t *testing.T) {
	tests := []struct {
		word, previous, next, want string
	}{
		{"read", "", "", "R IY1 D"},
		{"read", "please", "this", "R IY1 D"},
		{"read", "have", "this", "R EH1 D"},
		{"read", "i", "it", "R EH1 D"},
		{"live", "i", "in", "L IH1 V"},
		{"live", "the", "music", "L AY1 V"},
		{"lead", "the", "pipe", "L EH1 D"},
		{"lead", "", "the", "L IY1 D"},
		{"use", "the", "of", "Y UW1 S"},
		{"use", "i", "it", "Y UW1 Z"},
		{"record", "a", "song", "R EH1 K ER0 D"},
		{"record", "to", "a", "R AH0 K AO1 R D"},
		{"content", "is", "with", "K AH0 N T EH1 N T"},
		{"wind", "the", "", "W IH1 N D"},
		{"refuse", "the", "", "R EH1 F Y UW2 S"},
	}
	for _, tc := range tests {
		got, ok := heteronymReading(tc.word, tc.previous, tc.next)
		if !ok || got != tc.want {
			t.Errorf("%s after %q before %q = %q, %v; want %q", tc.word, tc.previous, tc.next, got, ok, tc.want)
		}
	}
}

func TestHeteronymSource(t *testing.T) {
	reading, err := ResolveInContext("read", "have", "this")
	if err != nil {
		t.Fatal(err)
	}
	if reading.Source != g2p.SourceHeteronym {
		t.Fatalf("source = %q, want %q", reading.Source, g2p.SourceHeteronym)
	}
}
