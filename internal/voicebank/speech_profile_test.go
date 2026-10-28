package voicebank

import (
	"github.com/yh2237/UtauIDX/audio"
	"github.com/yh2237/UtauTTS/internal/oto"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSpeechProfileBoundedAndInvalidated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.wav")
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
	for i := 800; i < 7200; i++ {
		pcm.Data[i] = int16(8000 * math.Sin(2*math.Pi*200*float64(i)/16000))
	}
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	entry := oto.Entry{Filename: path, Preutterance: 60, Fixed: 100}
	bank := &Bank{}
	profile := bank.CalibrateSpeech(entry)
	if !profile.Applied || math.Abs(profile.SuggestedFixedMS-100) > 20 || math.Abs(profile.F0Hz-200) > 5 {
		t.Fatalf("profile: %+v", profile)
	}
	if math.Abs(profile.TrimmedLengthMS-500) > 0.01 || math.Abs(profile.VowelTailMS-400) > 0.01 {
		t.Fatalf("source metrics: %+v", profile)
	}
	if profile.ActivityStartMS < 40 || profile.ActivityStartMS > 60 || profile.ActivityEndMS < 440 || profile.ActivityEndMS > 460 || profile.ActivityConfidence < .5 {
		t.Fatalf("activity: %+v", profile)
	}
	if profile.VoicingStartMS <= 0 || profile.TransitionEndMS < profile.TransitionStartMS || profile.TransitionConfidence <= 0 {
		t.Fatalf("voicing landmarks: %+v", profile)
	}
	for i := range pcm.Data {
		pcm.Data[i] = 0
	}
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	stamp := time.Now().Add(time.Second)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	silent := bank.CalibrateSpeech(entry)
	if silent.Applied || silent.SuggestedFixedMS != 100 || silent.Reason != "no-stable-voicing" {
		t.Fatalf("stale profile: %+v", silent)
	}
}

func TestMeasureSpeechTransientFindsLocalizedRelease(t *testing.T) {
	const rate = 8000
	samples := make([]float64, rate/5)
	for index := range samples {
		samples[index] = .02 * math.Sin(2*math.Pi*180*float64(index)/rate)
	}
	samples[rate*45/1000] = 1
	position, duration, confidence := measureSpeechTransient(samples, rate, 60)
	if math.Abs(position-45) > 3 || duration < 2 || duration > 8 || confidence < .5 {
		t.Fatalf("transient position=%f duration=%f confidence=%f", position, duration, confidence)
	}
}

func TestMeasureCodaBurst(t *testing.T) {
	const rate = 8000
	samples := make([]float64, rate*400/1000)
	for i := range samples {
		ms := float64(i) * 1000 / rate
		switch {
		case ms < 200:
			samples[i] = .5 * math.Sin(2*math.Pi*200*float64(i)/rate)
		case ms < 300:
			samples[i] = .02 * math.Sin(2*math.Pi*150*float64(i)/rate)
		case ms < 320:
			samples[i] = .3 * math.Sin(2*math.Pi*3000*float64(i)/rate)
		}
	}
	burst, confidence := measureCodaBurst(samples, rate, 200)
	if confidence < .5 || burst < 295 || burst > 310 {
		t.Fatalf("burst = %.1f (%.2f)", burst, confidence)
	}
	if burst, _ := measureCodaBurst(samples[:rate*290/1000], rate, 200); burst != 0 {
		t.Fatalf("burst without release = %.1f", burst)
	}
}
