package voicebank

import (
	"math"
	"math/rand"
	"path/filepath"
	"testing"
	"utautts/internal/audio"
	"utautts/internal/oto"
)

func TestSourceObservationSeparatesPeriodicNoiseAndSilence(t *testing.T) {
	for _, rate := range []int{16000, 44100} {
		x := make([]float64, rate)
		rng := rand.New(rand.NewSource(9))
		for i := range x {
			ms := float64(i) * 1000 / float64(rate)
			if ms >= 100 && ms < 400 {
				x[i] = .2 * math.Sin(2*math.Pi*180*float64(i)/float64(rate))
			}
			if ms >= 500 && ms < 800 {
				x[i] = .15 * (rng.Float64()*2 - 1)
			}
		}
		a := analyzeSourceFrames(x, rate)
		for _, tc := range []struct {
			ms   float64
			kind string
		}{{200, "periodic"}, {600, "aperiodic-high-crossing"}, {900, "low-energy"}} {
			found := false
			for _, f := range a.Frames {
				if f.StartMS <= tc.ms && f.EndMS > tc.ms {
					found = true
					if f.Evidence != tc.kind {
						t.Fatalf("%d Hz / %.0f ms: %+v", rate, tc.ms, f)
					}
				}
			}
			if !found {
				t.Fatal("missing frame")
			}
		}
		end := 0.0
		for _, region := range a.Regions {
			if region.StartMS != end || region.EndMS <= region.StartMS {
				t.Fatal(region)
			}
			end = region.EndMS
		}
		if math.Abs(end-a.DurationMS) > 1e-6 {
			t.Fatal("regions do not cover source")
		}
	}
}

func TestSourceObservationUsesOtoCoordinatesAndRejectsMissingSource(t *testing.T) {
	bank := &Bank{}
	if _, err := bank.AnalyzeSpeechSource(oto.Entry{Filename: "missing.wav"}); err == nil {
		t.Fatal("missing source accepted")
	}
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 16000)}
	for i := range pcm.Data {
		pcm.Data[i] = int16(4000 * math.Sin(2*math.Pi*200*float64(i)/16000))
	}
	path := filepath.Join(t.TempDir(), "source.wav")
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	entry := oto.Entry{Filename: path, Offset: 200, Blank: -300, Fixed: 100, Preutterance: 50}
	a, err := bank.AnalyzeSpeechSource(entry)
	if err != nil || math.Abs(a.DurationMS-300) > 1 || a.TimeOrigin != "oto-offset" || len(a.SourceSHA256) != 64 {
		t.Fatalf("%+v, %v", a, err)
	}
	if a.Status != "unverified-acoustic-observation" {
		t.Fatal("observation became verified label")
	}
}

func TestSourcePeriodicityRejectsDriftWithoutRejectingSpeechPitch(t *testing.T) {
	for _, rate := range []int{16000, 44100} {
		for _, hz := range []float64{30, 50, 80, 180, 300, 500} {
			x := make([]float64, rate/25)
			for i := range x {
				x[i] = .2 * math.Sin(2*math.Pi*hz*float64(i)/float64(rate))
			}
			got := sourcePeriodicity(x, rate)
			if hz < 80 && got >= .65 {
				t.Fatalf("%d Hz / %.0f Hz drift classified periodic: %.3f", rate, hz, got)
			}
			if hz >= 80 && got < .9 {
				t.Fatalf("%d Hz / %.0f Hz pitch rejected: %.3f", rate, hz, got)
			}
		}
		x := make([]float64, rate/25)
		for i := range x {
			x[i] = .1 + .2*float64(i)/float64(len(x))
		}
		if got := sourcePeriodicity(x, rate); got >= .65 {
			t.Fatalf("%d Hz ramp classified periodic: %.3f", rate, got)
		}
	}
}
