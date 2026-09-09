package voicebank

import (
	"errors"
	"testing"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

func TestCoverageContinuesWithContextAndPreservesStrictFailure(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{}}
	morae := []frontend.Mora{{Text: "か", Consonant: "k", Vowel: "a"}, {Text: "き", Consonant: "k", Vowel: "i"}, {Pause: true}, {Text: "く", Consonant: "k", Vowel: "u"}}
	coverage, err := bank.AuditCoverage(morae, "C4")
	if err != nil {
		t.Fatal(err)
	}
	if coverage.Positions != 3 || coverage.Covered != 0 || len(coverage.Missing) != 3 {
		t.Fatalf("coverage: %+v", coverage)
	}
	found := false
	for _, alias := range coverage.Missing[1].Candidates {
		if alias == "a き" {
			found = true
		}
	}
	if !found {
		t.Fatalf("previous vowel lost: %+v", coverage.Missing[1])
	}
	if coverage.Missing[2].Position != 3 {
		t.Fatal("pause changed indices")
	}
	_, err = bank.Resolve(morae)
	var missing *MissingAliasError
	if !errors.As(err, &missing) || missing.Position != 0 {
		t.Fatalf("strict failure changed: %v", err)
	}
}

func TestCoverageCountsClosureButNotPause(t *testing.T) {
	bank := &Bank{Entries: map[string][]oto.Entry{}}
	result, err := bank.AuditCoverage([]frontend.Mora{{Pause: true}, {Text: "っ", Vowel: "cl"}, {Text: "あ", Vowel: "a"}}, "C4")
	if err != nil {
		t.Fatal(err)
	}
	if result.Positions != 2 || result.Covered != 1 || result.CandidateCounts[1] != 1 || len(result.Missing) != 1 {
		t.Fatalf("coverage: %+v", result)
	}
}
