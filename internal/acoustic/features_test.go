package acoustic

import (
	"math"
	"testing"

	"utautts/internal/audio"
)

func TestAnalyzeFrameFindsPitchAndNormalizesSpectrum(t *testing.T) {
	const sampleRate = 16000
	values := make([]float64, sampleRate/5)
	for index := range values {
		values[index] = 0.25 * math.Sin(2*math.Pi*200*float64(index)/sampleRate)
	}
	got := AnalyzeFrame(values, sampleRate, 10, true)
	if !got.Valid || math.Abs(got.F0Hz-200) > 5 {
		t.Fatalf("frame = %#v", got)
	}
	mean := 0.0
	for _, value := range got.SpectrumDB {
		mean += value
	}
	if math.Abs(mean/float64(len(got.SpectrumDB))) > 1e-9 {
		t.Fatalf("normalized spectrum mean = %f", mean/float64(len(got.SpectrumDB)))
	}
}

func TestSpectralTiltDeltaTracksSpectralShapeDifference(t *testing.T) {
	flat := []float64{0, 0, 0, 0}
	mild := []float64{0, 0, 2, 2}
	strong := []float64{0, 0, 10, 10}
	falling := []float64{10, 10, 0, 0}
	if got := SpectralTiltDB(flat); math.Abs(got) > 1e-9 {
		t.Fatalf("flat tilt = %f, want 0", got)
	}
	if got := SpectralTiltDB(strong); math.Abs(got-10) > 1e-9 {
		t.Fatalf("strong tilt = %f, want 10", got)
	}
	mildDelta, strongDelta := SpectralTiltDelta(flat, mild), SpectralTiltDelta(flat, strong)
	if mildDelta >= strongDelta {
		t.Fatalf("mild=%f strong=%f, want monotonic increase", mildDelta, strongDelta)
	}
	if SpectralTiltDelta(flat, strong) != SpectralTiltDelta(flat, falling) {
		t.Fatalf("tilt delta must be symmetric")
	}
}

func TestMonoAveragesChannels(t *testing.T) {
	pcm := &audio.PCM{SampleRate: 8000, Channels: 2, Data: []int16{1000, 3000, -2000, 2000}}
	got := Mono(pcm)
	if len(got) != 2 || math.Abs(got[0]-2000.0/32768) > 1e-9 || got[1] != 0 {
		t.Fatalf("mono = %#v", got)
	}
}
