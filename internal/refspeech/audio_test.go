package refspeech

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
)

func TestAudioPitchRatio(t *testing.T) {
	manual := func(v float64) *float64 { return &v }
	for _, tc := range []struct {
		name                               string
		reference                          []float64
		bank                               []float32
		pitch                              *float64
		want, referenceMedian, indexMedian float64
	}{
		{"auto", []float64{0, 817, 439, 493, -1}, []float32{440, 0, 279, 306}, nil, 306.0 / 493, 493, 306},
		{"even", []float64{600, 0, 400}, []float32{200, 400}, nil, 0.6, 500, 300},
		{"lower clamp", []float64{1000}, []float32{100}, nil, 0.5, 1000, 100},
		{"upper clamp", []float64{100}, []float32{1000}, nil, 2, 100, 1000},
		{"empty index", []float64{500}, nil, nil, 1, 500, 0},
		{"unvoiced index", []float64{500}, []float32{0, -1}, nil, 1, 500, 0},
		{"unvoiced reference", []float64{0, -1}, []float32{300}, nil, 1, 0, 300},
		{"manual flat", []float64{500}, []float32{300}, manual(0), 1, 500, 300},
		{"manual down", []float64{500}, []float32{300}, manual(-8), math.Exp2(-8.0 / 12), 500, 300},
		{"manual up", []float64{500}, nil, manual(12), 2, 500, 0},
		{"manual clamp", nil, nil, manual(-24), 0.5, 0, 0},
		{"nonfinite f0", []float64{math.NaN(), math.Inf(1), 500}, []float32{300}, nil, 0.6, 500, 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reference, bank := slices.Clone(tc.reference), slices.Clone(tc.bank)
			got, refMedian, bankMedian, err := audioPitchRatio(tc.reference, tc.bank, tc.pitch)
			if err != nil || math.Abs(got-tc.want) > 1e-12 || refMedian != tc.referenceMedian || bankMedian != tc.indexMedian {
				t.Fatalf("got (%v, %v, %v, %v), want (%v, %v, %v)", got, refMedian, bankMedian, err, tc.want, tc.referenceMedian, tc.indexMedian)
			}
			sameReference := slices.EqualFunc(reference, tc.reference, func(a, b float64) bool {
				return a == b || math.IsNaN(a) && math.IsNaN(b)
			})
			if !sameReference || !slices.Equal(bank, tc.bank) {
				t.Fatal("input F0 was modified")
			}
		})
	}
	for _, pitch := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, _, _, err := audioPitchRatio(nil, nil, &pitch); err == nil {
			t.Fatal("accepted non-finite pitch")
		}
	}
}

func TestStandardizeAudioFeaturesSharedTransform(t *testing.T) {
	index := &refmatch.Index{
		Header:   refmatch.Header{Frames: 3, Dims: 2},
		Features: []float32{1, 0, 0, 1, -1, 0},
		File:     []int32{0, 0, 1},
	}
	query := []float32{2, 0}
	original := slices.Clone(index.Features)
	matched, got, err := standardizeAudioFeatures(index, query, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(matched.File, index.File) {
		t.Fatal("source recordings were lost during standardization")
	}
	want := []float32{float32(math.Sqrt(3) / 2), -0.5}
	for d := range want {
		if math.Abs(float64(got[d]-want[d])) > 1e-6 {
			t.Fatalf("query[%d] = %v, want %v", d, got[d], want[d])
		}
		if got[d] != matched.Features[d] {
			t.Fatalf("same direction transformed differently: query %v, bank %v", got, matched.Features[:2])
		}
	}
	for row := range matched.Frames {
		var norm float64
		for _, v := range matched.Features[row*2 : row*2+2] {
			norm += float64(v) * float64(v)
		}
		if math.Abs(norm-1) > 1e-6 {
			t.Fatalf("bank row %d norm squared = %v", row, norm)
		}
	}
	if !slices.Equal(index.Features, original) || !slices.Equal(query, []float32{2, 0}) {
		t.Fatal("original features were modified")
	}
}

func TestStandardizeAudioFeaturesConstantDimensions(t *testing.T) {
	index := &refmatch.Index{Header: refmatch.Header{Frames: 2, Dims: 2}, Features: []float32{1, 0, 1, 0}}
	matched, query, err := standardizeAudioFeatures(index, []float32{1, 0, 0, 1}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range [][]float32{matched.Features, query} {
		for _, v := range values {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				t.Fatalf("non-finite feature: %v", values)
			}
		}
	}
}

func TestStandardizeAudioFeaturesInvalidDimensions(t *testing.T) {
	for _, index := range []*refmatch.Index{
		{},
		{Header: refmatch.Header{Frames: 1, Dims: 2}},
		{Header: refmatch.Header{Frames: 1, Dims: 2}, Features: []float32{1, 0}},
	} {
		if _, _, err := standardizeAudioFeatures(index, []float32{1}, 1); err == nil {
			t.Fatal("accepted mismatched feature dimensions")
		}
	}
}

type audioTestWorld struct {
	f0 []float64
}

func (*audioTestWorld) Analyze([]float64, int, float64, float64) ([]float64, []float64, []float64, int, error) {
	panic("unexpected Analyze")
}

func (w *audioTestWorld) Synthesize(f0, spectrum, ap []float64, fftSize, sampleRate int, frameMS float64) ([]float64, error) {
	w.f0 = slices.Clone(f0)
	return []float64{0.1, -0.1}, nil
}

func TestSynthesizeAudioVoicingAndIndexIsolation(t *testing.T) {
	index := &refmatch.Index{
		Header:   refmatch.Header{Frames: 2, Dims: 2, SampleRate: 16000, FFTSize: 512, APBands: 1, F0Median: 200},
		Features: []float32{1, 0, 0, 1}, Spectrum: make([]float32, 2*4*60),
		Aperiodicity: make([]float32, 2*4), F0: []float32{200, 200, 200, 200, 0, 0, 0, 0}, File: []int32{0, 0},
	}
	features := slices.Clone(index.Features)
	f0 := []float64{100, 0, 0, 120, 0, 0, 0, 0}
	levels := []float64{0, 0, -40, 0, 0, 0, -40, 0}
	request := refmatch.Request{Features: features, Frames: 2, ReferenceF0: slices.Clone(f0)}
	// 元の索引のpackedキャッシュも先に作り、音声経路で置き換わらないことを確かめる。
	before, err := index.Parameters(request)
	if err != nil {
		t.Fatal(err)
	}
	world := &audioTestWorld{}
	var pitchLog string
	speech, err := synthesizeAudio(index, world, features, 2, f0, levels, AudioOptions{Logf: func(message string) { pitchLog = message }})
	if err != nil {
		t.Fatal(err)
	}
	if want := (VoicingStats{Frames: 8, Before: 2, After: 5}); *speech.ReferenceVoicing != want {
		t.Fatalf("voicing = %+v, want %+v", speech.ReferenceVoicing, want)
	}
	if f0[2] != 0 || f0[6] != 0 || f0[7] != 0 || f0[1] <= 0 || f0[4] <= 0 || f0[5] <= 0 {
		t.Fatalf("unexpected level-based filling: %v", f0)
	}
	// 中央値は音量による穴埋め前の参照（110 Hz）と索引の有声F0（200 Hz）から求める。
	wantF0 := slices.Clone(f0)
	for i := range wantF0 {
		wantF0[i] *= 200.0 / 110
	}
	if pitchLog == "" {
		t.Fatal("pitch correction was not logged")
	}
	if !slices.Equal(world.f0, wantF0) || !slices.Equal(speech.F0, wantF0) || !slices.Equal(speech.AutomaticF0, wantF0) {
		t.Fatalf("WORLD did not receive corrected output F0: %v, want %v", world.f0, wantF0)
	}
	if !slices.Equal(features, []float32{1, 0, 0, 1}) || !slices.Equal(index.Features, features) {
		t.Fatal("shared feature data was modified")
	}
	after, err := index.Parameters(request)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("original index search or waveform parameters changed")
	}
	flat := 0.0
	flatSpeech, err := synthesizeAudio(index, world, features, 2, slices.Clone(f0), levels, AudioOptions{PitchSemitones: &flat})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(world.f0, f0) || !slices.Equal(flatSpeech.F0, f0) {
		t.Fatalf("manual zero pitch did not preserve reference F0: %v, want %v", world.f0, f0)
	}
}
