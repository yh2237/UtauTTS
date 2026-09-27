package worldline

import (
	"math"
	"path/filepath"
	"reflect"
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
	got, err := placeSourceSpan(p, 0, item, span, 500, 0)
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
	t.Run("mapping precedence and fallback", func(t *testing.T) {
		newPlan := func() *plan.Plan {
			p := makePlan()
			p.Language = "en"
			p.PhoneTimings = []plan.PhoneTiming{{Position: 0, Role: "coda", Symbol: "k", StartMS: 100, DurationMS: 70}}
			return p
		}
		record := voicebank.SourcePhoneRecord{SourceSHA256: analysis.SourceSHA256, DurationMS: analysis.DurationMS, Phones: []voicebank.SourcePhoneInterval{{Symbol: "eh", StartMS: 0, EndMS: 210}, {Symbol: "k", StartMS: 210, EndMS: 340}, {Symbol: "eh", StartMS: 340, EndMS: 500}}}
		libraries := []*voicebank.SourcePhoneLibrary{{Entries: []voicebank.SourcePhoneRecord{record}}}
		p := newPlan()
		manual, err := mapSpeechSource(p, 0, item, base.WorldlineProviderOptions{ExperimentalSourceSpans: map[int]base.SourceSpan{0: span}}, libraries, 500, 0)
		if err != nil || p.Units[0].SpeechMapping != "experimental-aligned-source-span-v1" || !reflect.DeepEqual(manual.Speech, got.Speech) {
			t.Fatalf("manual mapping lost priority: %v %+v", err, manual.Speech)
		}
		p = newPlan()
		if _, err := mapSpeechSource(p, 0, item, base.WorldlineProviderOptions{}, libraries, 500, 0); err != nil || p.Units[0].SpeechMapping != "source-phone-library-v1" {
			t.Fatalf("library not applied: %v", err)
		}
		// ハッシュが一致しても不正な区間はoto推定へ戻す。
		libraries[0].Entries[0].Phones[1].EndMS = 600
		fallbackPlan := newPlan()
		fallback, err := mapSpeechSource(fallbackPlan, 0, item, base.WorldlineProviderOptions{}, libraries, 500, 0)
		baselinePlan := newPlan()
		baseline, baselineErr := placeSpeechUnit(baselinePlan, 0, item, 500, 0)
		if err != nil || baselineErr != nil || !reflect.DeepEqual(fallback, baseline) || !reflect.DeepEqual(fallbackPlan, baselinePlan) {
			t.Fatalf("invalid library changed fallback: %v %v", err, baselineErr)
		}
	})
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
			if _, err := placeSourceSpan(makePlan(), 0, item, s, 500, 0); err == nil {
				t.Fatal("invalid span accepted")
			}
		})
	}
}
