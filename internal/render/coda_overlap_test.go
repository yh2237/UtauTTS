package render

import (
	"math"
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

func TestCodaBoundaryLimitsNextOnsetIntrusion(t *testing.T) {
	units := []plan.Unit{
		{Role: "ending", NoteStartMS: 0, DurationMS: 65, PreutteranceMS: 250, OverlapMS: 83.333, CodaPhones: []string{"t"}},
		{Role: "mora", NoteStartMS: 65, DurationMS: 100, PreutteranceMS: 74.324, OverlapMS: 30},
	}
	timings, _ := openUtauPhoneTimingsWithCoda(units, "", true)
	if !timings[1].codaLimited {
		t.Fatal("coda boundary not limited")
	}
	tail := units[0].DurationMS - timings[0].tailIntrude + timings[0].tailOverlap
	if tail < codaBoundaryMinTailMS-1e-9 {
		t.Fatalf("audible coda tail = %v", tail)
	}
	// codaがなければ制限しない。
	units[0].CodaPhones = nil
	plain, _ := openUtauPhoneTimingsWithCoda(units, "", true)
	if plain[1].codaLimited || plain[1] == timings[1] {
		t.Fatalf("non-coda boundary changed: %+v", plain[1])
	}
}

func TestCodaBoundaryOverlapClampsAndSkipsShortCoda(t *testing.T) {
	// 語末子音が短すぎるときは何もしない。
	if _, _, limited := codaBoundaryOverlapMS(20, 100, 10); limited {
		t.Fatal("short coda must not be limited")
	}
	// 食い込みを末尾30msまでに抑え、overlapは上限へクランプする。
	preutterance, overlap, limited := codaBoundaryOverlapMS(65, 100, 40)
	if !limited || math.Abs(preutterance-35) > 1e-9 || math.Abs(overlap-codaBoundaryMaxOverlapMS) > 1e-9 {
		t.Fatalf("preutterance=%v overlap=%v limited=%v", preutterance, overlap, limited)
	}
	// 短いcodaでも重なり上限だけは守る。
	if _, overlap, limited := codaBoundaryOverlapMS(20, 100, 40); !limited || math.Abs(overlap-codaBoundaryMaxOverlapMS) > 1e-9 {
		t.Fatalf("short coda overlap=%v limited=%v", overlap, limited)
	}
	// すでに十分短い食い込みは変えない。
	if p, _, limited := codaBoundaryOverlapMS(65, 10, 5); limited || p != 10 {
		t.Fatalf("small intrusion changed: p=%v limited=%v", p, limited)
	}
}
