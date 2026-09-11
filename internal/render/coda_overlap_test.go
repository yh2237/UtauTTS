package render

import (
	"testing"
	"utautts/internal/plan"
)

func TestCodaOverlapReservesAudibleTail(t *testing.T) {
	units := []plan.Unit{{Role: "ending", NoteStartMS: 0, DurationMS: 41.263, PreutteranceMS: 250, OverlapMS: 83.333, CodaPhones: []string{"k"}},
		{Role: "ending", NoteStartMS: 41.263, DurationMS: 118, PreutteranceMS: 74.324, CodaPhones: []string{"s", "t"}}}
	before, _ := openUtauPhoneTimings(units, "")
	after, _ := openUtauPhoneTimingsWithCoda(units, "", true)
	oldTail := units[0].DurationMS - before[0].tailIntrude + before[0].tailOverlap
	tail := units[0].DurationMS - after[0].tailIntrude + after[0].tailOverlap
	if oldTail >= 10 || tail < 19.99 {
		t.Fatal(oldTail, tail)
	}
	units[0].CodaPhones = nil
	unchanged, _ := openUtauPhoneTimingsWithCoda(units, "", true)
	if unchanged[0] != before[0] || unchanged[1] != before[1] {
		t.Fatal("optional ending changed")
	}
}
