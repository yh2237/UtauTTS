package refspeech

import "testing"

func TestLeadingMSStartsWhereKokoroAudioStarts(t *testing.T) {
	if got := (Timing{Durations: []int{13, 2, 4}}).LeadingMS(); got != 13*DurationUnitMS-kokoroAudioLeadMS {
		t.Fatalf("leading = %v", got)
	}
	if got := (Timing{Durations: []int{1, 2}}).LeadingMS(); got != 0 {
		t.Fatalf("short leading = %v, want 0", got)
	}
}
