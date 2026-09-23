package connection

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/acoustic"
	"utautts/internal/audio"
	"utautts/internal/oto"
)

func TestBoundaryClampsFrameAtStartOfWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "start.wav")
	const sampleRate = 16000
	data := make([]int16, sampleRate/4)
	for index := range data {
		data[index] = int16(8000 * math.Sin(2*math.Pi*220*float64(index)/sampleRate))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	boundary := NewExtractor().Boundary(oto.Entry{Filename: path, Offset: 0, Preutterance: 10})
	if !boundary.Incoming.Valid {
		t.Fatal("incoming boundary at WAV start was not clamped to a valid frame")
	}
}

func TestPairMeasuresPitchSynchronousWaveformCorrelation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tone.wav")
	const sampleRate = 16000
	data := make([]int16, sampleRate/2)
	for index := range data {
		data[index] = int16(8000 * math.Sin(2*math.Pi*220*float64(index)/sampleRate))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	extractor := NewExtractor()
	left := oto.Entry{Filename: path, Offset: 20, Fixed: 60, Preutterance: 30, Overlap: 10}
	right := oto.Entry{Filename: path, Offset: 180, Fixed: 60, Preutterance: 30, Overlap: 10}
	features := extractor.Pair(left, right)
	if features.WaveformCorrelation < 0.95 {
		t.Fatalf("correlation=%f", features.WaveformCorrelation)
	}
}

func TestContextVCVAliasSoftensExpectedClosurePenalty(t *testing.T) {
	base := PairFeatures{
		PreviousOutgoing: acoustic.Frame{Valid: true, F0Hz: 220, RMSDB: -18},
		CurrentIncoming:  acoustic.Frame{Valid: true, F0Hz: 0, RMSDB: -42},
		SpectrumDelta:    20, RMSDelta: 20, VoicingMismatch: true,
	}
	withoutContext := HandcraftedScore(base)
	base.CurrentVCV = true
	withContext := HandcraftedScore(base)
	if withContext <= withoutContext {
		t.Fatalf("VCV closure score = %.3f, ordinary score = %.3f", withContext, withoutContext)
	}
}

func TestIsContextVCVAliasOnlyRecognizesKanaContexts(t *testing.T) {
	for _, test := range []struct {
		alias string
		want  bool
	}{
		{alias: "a か", want: true},
		{alias: "あ か", want: true},
		{alias: "- しょ C4", want: true},
		{alias: "a k", want: false},
		{alias: "か", want: false},
	} {
		if got := IsContextVCVAlias(test.alias); got != test.want {
			t.Fatalf("IsContextVCVAlias(%q) = %v, want %v", test.alias, got, test.want)
		}
	}
}

func TestSourceContinuityScoreConsidersAnchorDistance(t *testing.T) {
	near := PairFeatures{ForwardInSource: true, SourceAnchorDistanceMS: 100}
	far := PairFeatures{ForwardInSource: true, SourceAnchorDistanceMS: 2500}
	nearScore, farScore := HandcraftedScore(near), HandcraftedScore(far)
	if nearScore <= farScore {
		t.Fatalf("near=%f far=%f, want near > far", nearScore, farScore)
	}
	// 距離考慮でも連続性ボーナスは常に正の6〜9点に収める。
	if nearScore > 9 || farScore < 6 {
		t.Fatalf("forward continuity out of range: near=%f far=%f", nearScore, farScore)
	}
}

func TestHandcraftedScoreRewardsWaveformCorrelation(t *testing.T) {
	frame := acoustic.Frame{Valid: true, F0Hz: 220, RMSDB: -18}
	high := PairFeatures{PreviousOutgoing: frame, CurrentIncoming: frame, WaveformCorrelation: 1}
	low := PairFeatures{PreviousOutgoing: frame, CurrentIncoming: frame, WaveformCorrelation: 0}
	if HandcraftedScore(high) <= HandcraftedScore(low) {
		t.Fatalf("high=%f low=%f, want high > low", HandcraftedScore(high), HandcraftedScore(low))
	}
	if HandcraftedScore(high)-HandcraftedScore(low) > 4+1e-9 {
		t.Fatalf("correlation bonus exceeded bound: %f", HandcraftedScore(high)-HandcraftedScore(low))
	}
}

func TestHandcraftedScorePenalizesSpectralTiltDelta(t *testing.T) {
	frame := acoustic.Frame{Valid: true, F0Hz: 220, RMSDB: -18}
	small := PairFeatures{PreviousOutgoing: frame, CurrentIncoming: frame, SpectralTiltDelta: 1}
	large := PairFeatures{PreviousOutgoing: frame, CurrentIncoming: frame, SpectralTiltDelta: 100}
	if HandcraftedScore(large) >= HandcraftedScore(small) {
		t.Fatalf("small=%f large=%f, want small > large", HandcraftedScore(small), HandcraftedScore(large))
	}
	if HandcraftedScore(small)-HandcraftedScore(large) > 4+1e-9 {
		t.Fatalf("tilt penalty exceeded bound: %f", HandcraftedScore(small)-HandcraftedScore(large))
	}
}

func TestLegacyJoinCostIgnoresD1Features(t *testing.T) {
	features := PairFeatures{
		PreviousOutgoing:    acoustic.Frame{Valid: true, F0Hz: 220, RMSDB: -18},
		CurrentIncoming:     acoustic.Frame{Valid: true, F0Hz: 220, RMSDB: -18},
		WaveformCorrelation: 0, SpectralTiltDelta: 30,
	}
	previous := legacyJoinCost
	SetLegacyJoinCost(true)
	legacyScore := HandcraftedScore(features)
	if forward := HandcraftedScore(PairFeatures{ForwardInSource: true, SourceAnchorDistanceMS: 100}); forward != 8 {
		t.Fatalf("legacy forward score=%f, want 8", forward)
	}
	SetLegacyJoinCost(false)
	newScore := HandcraftedScore(features)
	SetLegacyJoinCost(previous)
	if newScore >= legacyScore {
		t.Fatalf("new=%f legacy=%f, want new < legacy", newScore, legacyScore)
	}
}
