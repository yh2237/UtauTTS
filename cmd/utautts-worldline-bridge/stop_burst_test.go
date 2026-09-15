package main

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/provider"
)

func TestHighPassKeepsStopTransient(t *testing.T) {
	samples := make([]float64, 1600)
	for index := range samples {
		samples[index] = .25 * math.Sin(2*math.Pi*180*float64(index)/16000)
	}
	samples[800] = 1
	transient := highPass(samples, 16000, 280)
	if math.Abs(transient[800]) < .5 {
		t.Fatalf("transient = %f, want a clear release peak", transient[800])
	}
}

func TestMixProtectedStopBurstAddsOnlyProtectedWindow(t *testing.T) {
	sourceSamples := make([]float64, 1600)
	sourceSamples[800] = 1
	source := protectedStopSource{sampleRate: 16000, samples: sourceSamples, transient: highPass(sourceSamples, 16000, 280)}
	wave := make([]float64, 1600)
	gain := mixProtectedStopBurst(wave, source, 42, 42, 8, 10, 100)
	if gain < .7 {
		t.Fatalf("localized burst gain = %f", gain)
	}
	if wave[800] == 0 {
		t.Fatal("protected burst was not restored")
	}
	if wave[0] != 0 || wave[len(wave)-1] != 0 {
		t.Fatal("protected burst leaked outside its window")
	}
}

func TestStopBurstLocalitySuppressesPersistentHighFrequency(t *testing.T) {
	localized := make([]float64, 1600)
	localized[800] = 1
	persistent := make([]float64, 1600)
	for index := range persistent {
		persistent[index] = .2 * math.Sin(2*math.Pi*2000*float64(index)/16000)
	}
	localScore := stopBurstLocality(localized, 16000, 42, 18)
	persistentScore := stopBurstLocality(persistent, 16000, 42, 18)
	if localScore <= persistentScore*3 || persistentScore < .8 || persistentScore > 1.2 {
		t.Fatalf("locality: localized=%f persistent=%f", localScore, persistentScore)
	}
}

func TestStopTransientPostMSUsesMeasuredDuration(t *testing.T) {
	if got := stopTransientPostMS(4, 32); got != 10 {
		t.Fatalf("short transient post = %f", got)
	}
	if got := stopTransientPostMS(40, 32); got != 24 {
		t.Fatalf("long transient post = %f", got)
	}
	if got := stopTransientPostMS(0, 32); got != 32 {
		t.Fatalf("missing transient post = %f", got)
	}
}

func TestStopBurstOnsetsPreserveDetectedSourceOffset(t *testing.T) {
	item := unit{OffsetMS: 3, PositionMS: 100, SkipMS: 5, Speech: &provider.WorldSpeechTiming{
		SourceOnsetMS: 60, SourceTransientMS: 45, TargetOnsetMS: 50,
	}}
	source, target := stopBurstOnsets(item, 10, 63, 50)
	if source != 48 || target != 130 {
		t.Fatalf("source=%f target=%f", source, target)
	}
}

func TestMixProtectedStopBurstsAlignsNonFrameOffset(t *testing.T) {
	const rate = 16000
	samples := make([]float32, rate/10)
	samples[rate*53/1000] = 1
	path := filepath.Join(t.TempDir(), "stop.wav")
	if err := writePCM16(path, rate, samples); err != nil {
		t.Fatal(err)
	}
	item := unit{
		Source: path, OffsetMS: 3, RequiredLengthMS: 100, ConsonantMS: 60,
		PositionMS: 0, SkipMS: 0, LengthMS: 100, Volume: 100,
		Speech: &provider.WorldSpeechTiming{SourceOnsetMS: 50, TargetOnsetMS: 50, ProtectStop: true},
	}
	wave := make([]float64, len(samples))
	input := manifest{SampleRate: rate, Units: []unit{item}}
	prepared := []preparedWorldUnit{{cached: cachedWorldUnit{duration: 100}}}
	mixProtectedStopBursts(input, prepared, wave, rate)
	if wave[rate*50/1000] == 0 {
		t.Fatal("burst was not aligned to the target onset")
	}
}

func TestMixProtectedStopBurstsSupportsPreserveOnlyTiming(t *testing.T) {
	const rate = 16000
	samples := make([]float32, rate/10)
	samples[rate*53/1000] = 1
	path := filepath.Join(t.TempDir(), "stop-preserve.wav")
	if err := writePCM16(path, rate, samples); err != nil {
		t.Fatal(err)
	}
	wave := make([]float64, len(samples))
	item := unit{Source: path, OffsetMS: 3, PositionMS: 0, SkipMS: 0, LengthMS: 100, Volume: 100,
		Speech: &provider.WorldSpeechTiming{SourceOnsetMS: 50, TargetOnsetMS: 50, ProtectStop: true, PreserveStopOnly: true}}
	mixProtectedStopBursts(manifest{SampleRate: rate, Units: []unit{item}}, []preparedWorldUnit{{cached: cachedWorldUnit{duration: 100}}}, wave, rate)
	if wave[rate*50/1000] == 0 {
		t.Fatal("preserve-only burst was not aligned to the target onset")
	}
}
