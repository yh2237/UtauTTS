package render

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/plan"
	"utautts/internal/voicebank"
)

func TestSpeechRetimePreservesStopReleaseAndLength(t *testing.T) {
	const rate = 16000
	source := make([]float64, msToFrames(400, rate))
	onset, fixed := msToFrames(60, rate), msToFrames(90, rate)
	for i := onset; i < len(source); i++ {
		source[i] = 0.2 * math.Sin(2*math.Pi*180*float64(i-onset)/rate)
	}
	// An isolated burst five milliseconds before vowel onset must survive
	// compression at the same offset from the target onset.
	source[onset-msToFrames(5, rate)] = 0.9
	targetOnset := msToFrames(35, rate)
	got, targetFixed, applied := speechRetime(source, msToFrames(240, rate), onset, fixed, targetOnset, msToFrames(60, rate), rate, true)
	if !applied || len(got) != msToFrames(240, rate) {
		t.Fatalf("applied=%v length=%d", applied, len(got))
	}
	if got[targetOnset-msToFrames(5, rate)] != 0.9 {
		t.Fatal("release burst moved or was attenuated")
	}
	if targetFixed <= targetOnset || targetFixed >= len(got) {
		t.Fatalf("invalid fixed anchor %d", targetFixed)
	}
	for _, v := range got {
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Abs(v) > 1 {
			t.Fatalf("invalid output %v", v)
		}
	}
}

func TestSpeechStopUsesMoraConsonantWithoutPhoneMetadata(t *testing.T) {
	p := &plan.Plan{Morae: []frontend.Mora{{Consonant: "k", Vowel: "a"}}}
	if !speechStop(p, plan.Unit{Position: 0}) {
		t.Fatal("stop consonant was not detected without phone metadata")
	}
	p.Morae[0].Consonant = "s"
	if speechStop(p, plan.Unit{Position: 0}) {
		t.Fatal("fricative was classified as a stop")
	}
}

func TestSpeechRetimeIdentityAndUncertainBoundaries(t *testing.T) {
	const rate = 16000
	source := make([]float64, 3200)
	for i := range source {
		source[i] = 0.2 * math.Sin(2*math.Pi*220*float64(i)/rate)
	}
	got, _, applied := speechRetime(source, len(source), 640, 960, 640, 960, rate, false)
	if !applied {
		t.Fatal("valid identity rejected")
	}
	for i := range source {
		if math.Abs(got[i]-source[i]) > 1e-12 {
			t.Fatalf("identity changed sample %d", i)
		}
	}
	for _, fixed := range []int{600, 640, 650, 4000} {
		if _, _, applied := speechRetime(source, 3200, 640, fixed, 640, 960, rate, false); applied {
			t.Fatalf("accepted ambiguous fixed=%d", fixed)
		}
	}
}

func TestSpeechAnchorFollowsChangingPitch(t *testing.T) {
	source := make([]float64, 1600)
	for i := range source {
		source[i] = float64(i) / float64(len(source))
	}
	curve := &PitchCurve{FrameMS: 10, Cents: []float64{0, 100, 200, 300, 400, 400, 300, 200, 100, 0}}
	warped := resampleForPitchCurve(source, 1, curve, 0, 100)
	for _, anchor := range []int{320, 800, 1200} {
		mapped := speechPitchAnchor(len(source), anchor, 1, curve, 0, 100)
		if math.Abs(warped[mapped]-source[anchor]) > 2/float64(len(source)) {
			t.Fatalf("pitch anchor %d moved to %d incorrectly", anchor, mapped)
		}
	}
}

func TestSpeechJoinProtectsConsonantsAndUnknownProfiles(t *testing.T) {
	p := &plan.Plan{Morae: []frontend.Mora{{Vowel: "a"}, {Vowel: "a"}}}
	left := renderedUnit{index: 0, unit: plan.Unit{Role: "mora", Position: 0, SpeechProfile: &voicebank.SpeechProfile{Applied: true}}}
	right := renderedUnit{index: 1, unit: plan.Unit{Role: "mora", Position: 1, SpeechProfile: &voicebank.SpeechProfile{Applied: true}}}
	if !speechVowelJoin(p, left, right) {
		t.Fatal("repeated vowel rejected")
	}
	p.Morae[1].Consonant = "k"
	if speechVowelJoin(p, left, right) {
		t.Fatal("stop accepted")
	}
	p.Morae[1].Consonant = ""
	p.Morae[0].Phones = []frontend.Phone{{Symbol: "s", Role: "coda"}}
	if speechVowelJoin(p, left, right) {
		t.Fatal("coda accepted")
	}
	p.Morae[0].Phones = nil
	p.Morae[1].Vowel = "i"
	if speechVowelJoin(p, left, right) {
		t.Fatal("different vowel accepted")
	}
	p.Morae[1].Vowel = "a"
	right.unit.SpeechProfile.Applied = false
	if speechVowelJoin(p, left, right) {
		t.Fatal("uncertain profile accepted")
	}
}

func TestSpeechRenderIsOptInAndKeepsNoteTiming(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.wav")
	data := make([]int16, 6400)
	for i := range data {
		data[i] = int16(5000 * math.Sin(2*math.Pi*180*float64(i)/16000))
	}
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	p := &plan.Plan{DurationMS: 140, Morae: []frontend.Mora{{Consonant: "k", Vowel: "a", Phones: []frontend.Phone{{Symbol: "k", Role: "onset"}, {Symbol: "a", Role: "nucleus"}}}}, Units: []plan.Unit{{Role: "mora", Position: 0, Source: path, DurationMS: 140, PreutteranceMS: 60, ConsonantMS: 90, SpeechProfile: &voicebank.SpeechProfile{Applied: true}}}}
	base, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	if p.Units[0].SpeechRetimeApplied {
		t.Fatal("disabled option applied")
	}
	p.SpeechTiming = true
	experiment, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	if !p.Units[0].SpeechRetimeApplied {
		t.Fatal("trusted timing not applied")
	}
	if len(base.Data) != len(experiment.Data) || p.Units[0].NoteStartMS != 0 || p.Units[0].DurationMS != 140 {
		t.Fatal("note timing or output length changed")
	}
	p.SpeechTiming = false
	again, err := Render(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	if p.Units[0].SpeechRetimeApplied {
		t.Fatal("stale diagnostic flag")
	}
	for i := range base.Data {
		if base.Data[i] != again.Data[i] {
			t.Fatalf("disabled output changed at %d", i)
		}
	}
	p.SpeechTiming = true
	reported, err := RenderWithReport(p, Config{ReleaseMS: 20})
	if err != nil {
		t.Fatal(err)
	}
	if p.Units[0].SpeechRetimeApplied || !reported.Report.Units[0].SpeechRetimeApplied {
		t.Fatal("renderer diagnostics mutated the selection plan or were lost")
	}
	exported := plan.Clone(p)
	reported.Report.ApplyTo(exported)
	if !exported.Units[0].SpeechRetimeApplied {
		t.Fatal("speech timing diagnostic missing from export")
	}
}
