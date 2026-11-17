package f0cache

import (
	"math"
	"path/filepath"
	"reflect"
	"testing"
)

type timedToken struct {
	start, end float64
	pause      bool
}

func (t timedToken) F0Timing() (float64, float64, bool) {
	return t.start, t.end, t.pause
}

func TestLegacyCacheKeys(t *testing.T) {
	for _, tc := range []struct {
		id, recordID string
		tokens       []timedToken
		step         float64
		source, want string
	}{
		{"sample", "", []timedToken{{0, 100, false}, {100, 200, true}}, 10, WorldHarvest, "20d4c32b3904508b1e562a6ca0a9abbd87954aa6.npy"},
		{"sample", "", []timedToken{{0, 100, false}, {100, 200, true}}, 2.5, "internal_autocorrelation", "0afa7f8efaf0e46503f721189197c3c243deda9f.npy"},
		{"segment", "音声|source", []timedToken{{-0.0004, 12.3455, false}, {12.3456, 25.0004, true}}, 10, WorldHarvest, "5eb550bfe710cfc2966f7dd0f74f15571eb47b25.npy"},
		{"empty", "", nil, 10, WorldHarvest, "be38d3835b3fab873a4bec551c2e0461308dd968.npy"},
	} {
		if got := Path("cache", tc.id, tc.recordID, tc.tokens, tc.step, tc.source); got != filepath.Join("cache", tc.want) {
			t.Errorf("%s %g %s: %s", tc.id, tc.step, tc.source, got)
		}
	}
}

func TestTimeGridAndTokenSelection(t *testing.T) {
	for _, tc := range []struct {
		start, end, step float64
		tokens           []timedToken
		want             []float64
	}{
		{3, 24, 10, nil, []float64{8, 18, 28}},
		{30, 20, 10, []timedToken{{2, 13, false}, {20, 24, true}}, []float64{7, 17, 27}},
		{0, 0, 10, []timedToken{{7, 7, false}}, []float64{12}},
		{0, 0, 10, []timedToken{{9, 3, false}}, []float64{14}},
		{1, 6, 2.5, nil, []float64{2.25, 4.75}},
	} {
		if got := TimeGrid(tc.start, tc.end, tc.tokens, tc.step); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("grid = %v, want %v", got, tc.want)
		}
	}
	tokens := []timedToken{{10, 20, false}, {30, 30, true}, {40, 50, false}}
	for _, tc := range []struct {
		time float64
		want int
	}{{9, 0}, {10, 0}, {20, 2}, {29, 2}, {30, 1}, {30.0000005, 1}, {30.000001, 2}, {40, 2}, {50, 2}} {
		if got := TokenAt(tokens, tc.time); got != tc.want {
			t.Errorf("token at %g = %d, want %d", tc.time, got, tc.want)
		}
	}
}

func TestLegacyInterpolation(t *testing.T) {
	track := []float64{100, 400, 0, -25, 1600, 6400}
	times := []float64{-1, 0, 5, 10, 15, 20, 25, 30, 35, 40, 45, 50, 55, 60, 10 + 1e-10}
	want := []float64{0, 100, 200.00000000000009, 400, 0, 0, 0, -25, 0, 1600, 3199.9999999999973, 6400, 0, 0, 400}
	for _, step := range []float64{10, 2.5} {
		queries := make([]float64, len(times))
		for i, x := range times {
			queries[i] = x * step / 10
		}
		for i, x := range Interpolate(track, queries, step) {
			tolerance := 0.0
			if times[i] == 5 || times[i] == 45 {
				tolerance = math.Abs(want[i]) * 1e-12
			}
			if math.IsNaN(x) || math.Abs(x-want[i]) > tolerance {
				t.Errorf("step %g at %g = %.17g, want %.17g", step, queries[i], x, want[i])
			}
		}
	}
}
