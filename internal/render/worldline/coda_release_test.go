package worldline

import (
	"math"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func TestMainCodaFadeScope(t *testing.T) {
	for _, phonemizer := range []string{frontend.PhonemizerChinese} {
		p := &plan.Plan{Phonemizer: phonemizer}
		u := plan.Unit{Role: "mora", CodaPhones: []string{"n"}, DurationMS: 60}
		if !worldCodaReleaseEligible(p, u) {
			t.Fatal(phonemizer)
		}
		_, fade := codaReleaseEnvelope(u, nil, 20)
		if fade > 5 {
			t.Fatal("coda faded away", fade)
		}
		u.Role = "ending"
		if worldCodaReleaseEligible(p, u) {
			t.Fatal("optional ending protected", phonemizer)
		}
	}
}

func TestCodaReleaseEnvelopeKeepsConsonantAudible(t *testing.T) {
	u := plan.Unit{DurationMS: 30, CodaPhones: []string{"d"}}
	points := []base.WorldlineEnvelopePoint{{XMS: -30}, {XMS: -20}, {XMS: 0}, {XMS: 0}, {XMS: 30}}
	got, fade := codaReleaseEnvelope(u, points, 30)
	if fade != 5 || got[3].XMS != 25 || got[4].XMS != 30 {
		t.Fatalf("fade=%v points=%#v", fade, got)
	}
}

func TestCodaClosureReleaseSplitSeparatesStop(t *testing.T) {
	u := plan.Unit{Role: "ending", CodaPhones: []string{"t"}, DurationMS: 70,
		SpeechProfile: &voicebank.SpeechProfile{ReleaseTransientMS: 40, ReleaseTransientDurationMS: 16, ReleaseTransientConfidence: .6}}
	closure, release, ok := codaClosureReleaseSplit(u)
	if !ok {
		t.Fatal("stop coda not split")
	}
	if math.Abs(closure+release-u.DurationMS) > 1e-9 {
		t.Fatalf("total length changed: closure=%v release=%v", closure, release)
	}
	if release < codaReleaseMinMS || release > codaReleaseMaxMS || closure < codaClosureMinMS {
		t.Fatalf("unbounded split: closure=%v release=%v", closure, release)
	}
	if _, _, ok := codaClosureReleaseSplit(plan.Unit{CodaPhones: []string{"s"}, DurationMS: 70}); ok {
		t.Fatal("fricative coda must not split")
	}
	if _, _, ok := codaClosureReleaseSplit(plan.Unit{CodaPhones: []string{"t"}, DurationMS: 20}); ok {
		t.Fatal("short coda must not split")
	}
}
