package worldline

import (
	"math"
	"path/filepath"
	"testing"
	"utautts/internal/audio"
	"utautts/internal/oto"
	"utautts/internal/plan"
	"utautts/internal/render/base"
	"utautts/internal/voicebank"
)

func TestExperimentalSpanExcludesFollowingVowelAndKeepsBurst(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.wav")
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
	for i := range pcm.Data {
		pcm.Data[i] = int16(10000 * math.Sin(float64(i)*.1))
	}
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	bank := &voicebank.Bank{}
	analysis, err := bank.AnalyzeSpeechSource(oto.Entry{Filename: path})
	if err != nil {
		t.Fatal(err)
	}
	span := base.ExperimentalSourceSpan{Alias: "e k", SourceSHA256: analysis.SourceSHA256, CoreStartMS: 210, CoreEndMS: 340, ContextStartMS: 0,
		Mappings:  []base.ExperimentalPhoneMapping{{Symbol: "k", SourceStartMS: 210, SourceEndMS: 340, RequestedStartMS: 100, RequestedEndMS: 170}},
		Landmarks: []base.ExperimentalLandmark{{Kind: "preutterance-transient", SourceMS: 300, DurationMS: 5, Score: .9, RelativeDB: -10}, {Kind: "energy-rise", SourceMS: 340, DurationMS: 20, Score: 1, RelativeDB: 0}}}
	makePlan := func() *plan.Plan {
		return &plan.Plan{Units: []plan.Unit{{Alias: "e k", Role: "ending", Source: path, NoteStartMS: 100, DurationMS: 70, CodaPhones: []string{"k"}}}}
	}
	item := worldlineManifestUnit{PositionMS: 80, LengthMS: 90}
	p := makePlan()
	got, err := placeExperimentalSourceSpan(p, 0, item, span, 500, 0)
	if err != nil {
		t.Fatal(err)
	}
	a := got.Speech.Anchors
	if a[0].SourceMS != 170 || a[len(a)-1].SourceMS != 340 || a[len(a)-1].TargetMS != 90 || !got.Speech.ProtectStop || got.Speech.SourceTransientMS != 300 {
		t.Fatalf("wrong mapping: %+v", got.Speech)
	}
	protected := false
	for i := 1; i < len(a); i++ {
		if a[i-1].SourceMS == 296 {
			protected = true
			if math.Abs((a[i].SourceMS-a[i-1].SourceMS)-(a[i].TargetMS-a[i-1].TargetMS)) > .001 {
				t.Fatal("burst speed changed")
			}
		}
	}
	if !protected {
		t.Fatal("missing burst anchors")
	}
	for _, kind := range []string{"hash", "alias", "timing", "bounds", "nan"} {
		t.Run(kind, func(t *testing.T) {
			s := span
			s.Mappings = append([]base.ExperimentalPhoneMapping{}, span.Mappings...)
			switch kind {
			case "hash":
				s.SourceSHA256 = "stale"
			case "alias":
				s.Alias = "other"
			case "timing":
				s.Mappings[0].RequestedStartMS = 110
			case "bounds":
				s.CoreEndMS = 600
			case "nan":
				s.Mappings[0].SourceStartMS = math.NaN()
			}
			if _, err := placeExperimentalSourceSpan(makePlan(), 0, item, s, 500, 0); err == nil {
				t.Fatal("invalid span accepted")
			}
		})
	}
}
