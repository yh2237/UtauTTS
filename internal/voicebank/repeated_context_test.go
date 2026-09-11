package voicebank

import (
	"path/filepath"
	"testing"
	"utautts/internal/audio"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

func TestRepeatedContextUsesThreeOrderedAnchors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "he_he_he.wav")
	current := frontend.Mora{Text: "he", Language: frontend.LanguageChinese, Phones: []frontend.Phone{{Symbol: "h", Role: "onset"}}}
	previous := frontend.Mora{Text: "he", Vowel: "e"}
	first := oto.Entry{Filename: path, Offset: 0, Preutterance: 20}
	last := oto.Entry{Filename: path, Offset: 450, Preutterance: 20}
	vc := oto.Entry{Filename: path, Offset: 50, Preutterance: 70, Fixed: 90, Blank: 300}
	b := &Bank{Entries: map[string][]oto.Entry{"- he": {first}, "he": {last}}}
	if limit, ok := b.repeatedContextLimit(vc, current, previous); !ok || limit != 450 {
		t.Fatal(limit, ok)
	}
	pcm := &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 8000)}
	for i, v := range contextTestWave() {
		pcm.Data[i] = int16(v * 32767)
	}
	if err := audio.WriteWav(path, pcm); err != nil {
		t.Fatal(err)
	}
	s := Selection{Position: 1, Mora: current, Entry: last, Composite: true, Transition: &Selection{Entry: vc}}
	got := b.ApplyRepeatedContext([]frontend.Mora{previous, current}, []Selection{s})[0]
	if got.SourceContext != "recovered-repeat" || got.Transition != nil || got.Entry.Offset-got.Entry.Blank > 450 {
		t.Fatal(got)
	}
	if s.Entry != last || s.Transition.Entry != vc || b.Entries["he"][0] != last {
		t.Fatal("input modified")
	}
	delete(b.Entries, "- he")
	if _, ok := b.repeatedContextLimit(vc, current, previous); ok {
		t.Fatal("missing first anchor accepted")
	}
	b.Entries["- he"] = []oto.Entry{first}
	b.Entries["he"][0].Offset = 100
	if _, ok := b.repeatedContextLimit(vc, current, previous); ok {
		t.Fatal("unordered anchors accepted")
	}
	wrong := current
	wrong.Text = "hao"
	if _, ok := b.repeatedContextLimit(vc, wrong, previous); ok {
		t.Fatal("wrong syllable accepted")
	}
}
