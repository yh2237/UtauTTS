package speechtiming

import (
	"encoding/json"
	"math"
	"os"
	"reflect"
	"testing"
)

func TestDefaultTargetMatchesPyTorch(t *testing.T) {
	data, err := os.ReadFile("testdata/target-v1-parity.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		IDs    [][3]int     `json:"ids"`
		Cont   [][4]float32 `json:"cont"`
		Output [][]float32  `json:"output"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	model, err := DefaultTarget()
	if err != nil {
		t.Fatal(err)
	}
	got, err := model.Predict(fixture.IDs, fixture.Cont)
	if err != nil {
		t.Fatal(err)
	}
	worst := 0.0
	for frame := range fixture.Output {
		for channel, want := range fixture.Output[frame] {
			worst = math.Max(worst, math.Abs(float64(got[frame][channel]-want)))
		}
	}
	if worst > 1e-3 {
		t.Fatalf("max abs difference from PyTorch = %g", worst)
	}
}

func TestMoraPhones(t *testing.T) {
	cases := map[string][]string{
		"か": {"k", "a"}, "し": {"sh", "i"}, "つ": {"ts", "u"}, "ふ": {"f", "u"}, "じ": {"j", "i"},
		"きょ": {"ky", "o"}, "あ": {"a"}, "ん": {"N"}, "っ": {"cl"}, "を": {"o"},
		"てぃ": {"t", "i"}, "ふぁ": {"f", "a"}, "うぃ": {"w", "i"}, "ゔ": {"v", "u"},
	}
	for text, want := range cases {
		if got := moraPhones(text, ""); !reflect.DeepEqual(got, want) {
			t.Errorf("moraPhones(%q) = %v, want %v", text, got, want)
		}
	}
	if got := moraPhones("ー", "o"); !reflect.DeepEqual(got, []string{"o"}) {
		t.Errorf("long vowel = %v", got)
	}
	model, err := DefaultTarget()
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, phone := range model.Phones() {
		known[phone] = true
	}
	for text := range cases {
		for _, phone := range moraPhones(text, "") {
			if !known[phone] {
				t.Errorf("phone %q of %q is not in the model", phone, text)
			}
		}
	}
}

func TestPhoneTimelineMarksPhraseEnds(t *testing.T) {
	morae := []Mora{
		{Text: "か", NoteStartMS: 0, DurationMS: 120, EffectivePreutteranceMS: 40},
		{Text: "な", NoteStartMS: 120, DurationMS: 120, EffectivePreutteranceMS: 30},
		{Text: "た", NoteStartMS: 400, DurationMS: 120, EffectivePreutteranceMS: 40},
	}
	phones, starts, ends := phoneTimeline(morae, 50, 80)
	if !reflect.DeepEqual(starts, []float64{0.05, 0.17, 0.45}) {
		t.Fatalf("starts = %v", starts)
	}
	if len(ends) != 2 || math.Abs(ends[0]-0.29) > 1e-9 || math.Abs(ends[1]-0.57) > 1e-9 {
		t.Fatalf("ends = %v", ends)
	}
	if phones[0].label != "sil" || phones[1].label != "k" || phones[2].label != "a" {
		t.Fatalf("phones = %v", phones[:3])
	}
}

func TestPhoneTimelineUsesExplicitSpans(t *testing.T) {
	morae := []Mora{{Text: "w3", NoteStartMS: 0, DurationMS: 120, EffectivePreutteranceMS: 40,
		Spans: []PhoneSpan{
			{Label: "w", StartMS: 0, DurationMS: 30},
			{Label: "er", StartMS: 30, DurationMS: 50},
			{Label: "l", StartMS: 80, DurationMS: 20},
			{Label: "d", StartMS: 100, DurationMS: 20},
		}}}
	phones, starts, ends := phoneTimeline(morae, 50, 30)
	var labels []string
	for _, phone := range phones {
		labels = append(labels, phone.label)
	}
	want := []string{"sil", "w", "er", "l", "d", "sil"}
	if !reflect.DeepEqual(labels, want) {
		t.Fatalf("labels = %v, want %v", labels, want)
	}
	if len(starts) != 1 || math.Abs(starts[0]-0.05) > 1e-9 {
		t.Fatalf("starts = %v", starts)
	}
	if phones[1].start != 0.05 || phones[1].end != 0.08 {
		t.Fatalf("w span = %v..%v", phones[1].start, phones[1].end)
	}
	if len(ends) != 1 || math.Abs(ends[0]-0.17) > 1e-9 {
		t.Fatalf("ends = %v", ends)
	}
}

func TestTargetForLanguageLoadsEnglishModel(t *testing.T) {
	english, err := TargetForLanguage("en")
	if err != nil {
		t.Fatal(err)
	}
	if len(english.Phones()) != 42 {
		t.Fatalf("English phones = %d, want 42", len(english.Phones()))
	}
	known := map[string]bool{}
	for _, phone := range english.Phones() {
		known[phone] = true
	}
	if !known["ah"] || !known["ow"] || !known["sil"] {
		t.Fatalf("English vocabulary is missing ARPAsing symbols: %v", english.Phones())
	}
	japanese, err := TargetForLanguage("ja")
	if err != nil {
		t.Fatal(err)
	}
	if len(japanese.Phones()) != 40 || japanese == english {
		t.Fatalf("Japanese model = %d phones, same=%v", len(japanese.Phones()), japanese == english)
	}
}

func TestWarpKeepsIdenticalTargetAndProtectsEnds(t *testing.T) {
	frames := 60
	rows := make([][]float64, frames)
	for frame := range rows {
		rows[frame] = []float64{math.Sin(float64(frame) * 0.3), math.Cos(float64(frame) * 0.17)}
	}
	mapping := warpMap(rows, rows, []float64{20, 40}, frames)
	for frame, value := range mapping {
		if math.Abs(value-float64(frame)) > 1e-9 {
			t.Fatalf("identity mapping[%d] = %v", frame, value)
		}
	}
	weight := protectPhraseEnds(100, []float64{0.1, 0.3, 0.5}, []float64{0.62})
	if weight[20] != 1 || weight[38] != 0 || weight[64] != 0 || weight[65] != 1 {
		t.Fatalf("weights = %v", weight)
	}
	if weight[35] <= 0 || weight[35] >= 1 {
		t.Fatalf("ramp weight = %v", weight[35])
	}
}

func TestPhoneTimelineKeepsConsonantsInContinuousSpeech(t *testing.T) {
	morae := []Mora{
		{Text: "あ", NoteStartMS: 0, DurationMS: 120},
		{Text: "さ", NoteStartMS: 120, DurationMS: 120, EffectivePreutteranceMS: 70},
	}
	phones, _, _ := phoneTimeline(morae, 100, 60)
	var found bool
	for i, phone := range phones {
		if phone.label == "s" {
			found = true
			if math.Abs(phone.start-0.15) > 1e-9 || math.Abs(phone.end-0.22) > 1e-9 {
				t.Fatalf("s span = %v..%v, want 0.15..0.22", phone.start, phone.end)
			}
			if previous := phones[i-1]; previous.label != "a" || math.Abs(previous.end-0.15) > 1e-9 {
				t.Fatalf("previous vowel = %+v, want a ending at 0.15", previous)
			}
		}
	}
	if !found {
		t.Fatalf("consonant missing: %+v", phones)
	}
}
