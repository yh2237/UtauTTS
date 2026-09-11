package voicebank

import (
	"math"
	"path/filepath"
	"testing"
	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

func contextTestWave() []float64 {
	w := make([]float64, 8000)
	for i := range w {
		if i < 1600 || i >= 3200 {
			w[i] = .3 * math.Sin(2*math.Pi*200*float64(i)/16000)
		}
	}
	return w
}

func TestRecoverVowelRequiresBoundaryAndExtension(t *testing.T) {
	w := contextTestWave()
	for _, blank := range []float64{300, -150} {
		onset, end, ok := recoverVowelAfterVC(w, 16000, oto.Entry{Offset: 50, Preutterance: 70, Fixed: 90, Blank: blank})
		if !ok || onset < 180 || onset > 225 || end < 350 {
			t.Fatalf("%v: %v %v %v", blank, onset, end, ok)
		}
	}
	for i := range w {
		w[i] = .3 * math.Sin(2*math.Pi*200*float64(i)/16000)
	}
	if _, _, ok := recoverVowelAfterVC(w, 16000, oto.Entry{Blank: 300}); ok {
		t.Fatal("accepted no boundary")
	}
	if _, _, ok := recoverVowelAfterVC(make([]float64, 8000), 16000, oto.Entry{Blank: 300}); ok {
		t.Fatal("accepted silence")
	}
}

func TestRecordedContextOnlyRecoversMatchingSource(t *testing.T) {
	previous := frontend.Mora{Text: "ni", Vowel: "i"}
	current := frontend.Mora{Text: "he", Language: frontend.LanguageChinese, Phones: []frontend.Phone{{Symbol: "h", Role: "onset"}}}
	path := filepath.Join(t.TempDir(), "i_he.wav")
	vc := oto.Entry{Filename: path, Offset: 50, Preutterance: 70, Fixed: 90, Blank: -150}
	b := &Bank{}
	if !b.matchesContextRecording(vc, current, previous) {
		t.Fatal("matching source rejected")
	}
	wrong := current
	wrong.Text = "hao"
	if b.matchesContextRecording(vc, wrong, previous) {
		t.Fatal("wrong vowel accepted")
	}
	for _, name := range []string{"he_he_he.wav", "unknown.wav", "a_he.wav"} {
		entry := vc
		entry.Filename = name
		if b.matchesContextRecording(entry, current, previous) {
			t.Fatal("ambiguous source accepted", name)
		}
	}
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
	for i, v := range contextTestWave() {
		pcm.Data[i] = int16(v * 32767)
	}
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	s := Selection{Position: 1, Mora: current, Entry: oto.Entry{Filename: "original.wav"}, Composite: true, Transition: &Selection{Entry: vc}}
	got := b.ApplyRecordedContext([]frontend.Mora{previous, current}, []Selection{s}, true)[0]
	if got.SourceContext != "recovered-vc" || got.Transition != nil || got.Composite || got.Entry.Filename != path || got.Kind != AliasVCV {
		t.Fatalf("%+v", got)
	}
	if s.Transition.Entry != vc || s.Entry.Filename != "original.wav" {
		t.Fatal("input mutated")
	}
	if got.Entry.Offset != vc.Offset || got.Entry.Blank >= vc.Blank {
		t.Fatal("not extended")
	}
	if got := b.ApplyRecordedContext([]frontend.Mora{previous, current}, []Selection{s}, false)[0]; got.Entry != s.Entry {
		t.Fatal("existing mode recovered audio")
	}
}
