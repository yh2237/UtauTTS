package voicebank

import (
	"testing"
	"utautts/internal/connection"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

func TestEndingRecordingsOptimizeJoinsAndRespectMissingPhoneGap(t *testing.T) {
	left := Selection{Alias: "first", TargetScore: 100, CodaStart: 0, CodaPhones: []string{"k"}, Entry: oto.Entry{SourceGroup: "A"}}
	right := Selection{Alias: "isolated-best", TargetScore: 100, CodaStart: 1, CodaPhones: []string{"s"}, Entry: oto.Entry{SourceGroup: "B"}}
	compatible := right
	compatible.Alias, compatible.TargetScore, compatible.Entry.SourceGroup = "compatible", 98, "A"
	layers := [][]Selection{{left}, {right, compatible}}
	path := selectEndingRecordings(layers, connection.NewExtractor())
	if path[1].Alias != "compatible" {
		t.Fatalf("join did not influence choice: %+v", path)
	}
	badLeft := left
	badLeft.Alias, badLeft.TargetScore, badLeft.Entry.SourceGroup = "isolated-first", 101, "B"
	whole := selectEndingRecordings([][]Selection{{badLeft, left}, {compatible}}, connection.NewExtractor())
	if whole[0].Alias != "first" {
		t.Fatal("path optimization kept a greedy first choice")
	}
	main := Selection{Entry: oto.Entry{SourceGroup: "A"}}
	firstOnly := selectEndingRecordingsFrom(&main, [][]Selection{{badLeft, left}}, connection.NewExtractor())
	if firstOnly[0].Alias != "first" {
		t.Fatal("main-to-ending compatibility was ignored")
	}
	layers[1][0].CodaStart, layers[1][1].CodaStart = 2, 2
	path = selectEndingRecordings(layers, connection.NewExtractor())
	if path[1].Alias != "isolated-best" {
		t.Fatal("missing phone gap received a join penalty")
	}
}

func TestUtteranceGraphCanChooseEndingForFollowingRecording(t *testing.T) {
	a := Selection{Alias: "locally-best", TargetScore: 100, CodaPhones: []string{"d"}, Entry: oto.Entry{SourceGroup: "A"}}
	b := a
	b.Alias, b.TargetScore, b.Entry.SourceGroup = "next-compatible", 99, "B"
	main := Selection{Mora: frontend.Mora{Language: "en"}, TargetScore: 100, EndingCandidates: [][]Selection{{a, b}}}
	next := Selection{Mora: frontend.Mora{Language: "en"}, TargetScore: 100, Entry: oto.Entry{SourceGroup: "B"}}
	path := selectBestPaths([][]Selection{{main}, {next}}, connection.NewExtractor())
	if path[0].Endings[0].Alias != "next-compatible" {
		t.Fatal("terminal alternatives were discarded before the following join")
	}
}
