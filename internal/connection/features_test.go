package connection

import (
	"math"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauIDX/audio"
	"github.com/yh2237/UtauTTS/internal/acoustic"
	"github.com/yh2237/UtauTTS/internal/oto"
)

func TestSpeechContextUsesRetainedRegionInsteadOfOtoOverlap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "context.wav")
	const rate = 16000
	data := make([]int16, rate/2)
	for i := range data {
		frequency := 220.0
		if i > rate/4 {
			frequency = 330
		}
		data[i] = int16(8000 * math.Sin(2*math.Pi*frequency*float64(i)/rate))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: rate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	cache := NewExtractor()
	previous := oto.Entry{Filename: path, Fixed: 60, Preutterance: 30}
	current := oto.Entry{Filename: path, Offset: 180, Preutterance: 100, Overlap: 90}
	wantEntry := current
	wantEntry.Overlap = 20
	want := cache.ScoreEntries(previous, wantEntry)
	got := cache.ScoreSpeechContext(previous, current)
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %g, want %g", got, want)
	}
	if math.Abs(got-cache.ScoreEntries(previous, current)) < .1 {
		t.Fatal("fixture did not distinguish retained context from legacy overlap")
	}
	current.Overlap = 0
	if other := cache.ScoreSpeechContext(previous, current); math.Abs(got-other) > 1e-9 {
		t.Fatalf("speech score depends on unused overlap: %g vs %g", got, other)
	}
	current.Preutterance = 10
	wantEntry = current
	wantEntry.Overlap = 5
	if got, want := cache.ScoreSpeechContext(previous, current), cache.ScoreEntries(previous, wantEntry); math.Abs(got-want) > 1e-9 {
		t.Fatalf("short context: %g vs %g", got, want)
	}
}

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
