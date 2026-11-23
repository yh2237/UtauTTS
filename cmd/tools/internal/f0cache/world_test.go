package f0cache

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauTTS/internal/wavfile"
	"github.com/yh2237/UtauWLD/runtimepath"
	"github.com/yh2237/UtauWLD/worldrender"
	"github.com/yh2237/audiodsp/wav"
)

func TestWorldF0NativeReferenceParity(t *testing.T) {
	enginePath, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "runtime", runtimepath.LibraryName("utautts-world-engine")))
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(enginePath)
	if os.IsNotExist(err) {
		t.Skipf("native WORLD engine is not installed: %s", enginePath)
	}
	if err != nil {
		t.Fatalf("stat native WORLD engine: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("native WORLD engine is not a regular file: %s", enginePath)
	}
	world, err := worldrender.OpenWorld(enginePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := world.Close(); err != nil {
			t.Error(err)
		}
	})

	harmonics := func(hz, seconds float64) float64 {
		var sample float64
		for harmonic := 1; harmonic <= 6; harmonic++ {
			sample += .2 / float64(harmonic) * math.Sin(2*math.Pi*hz*float64(harmonic)*seconds)
		}
		return sample
	}
	for _, tc := range []struct {
		name        string
		rate, count int
		channels    int
		antiPhase   bool
		signal      func(float64) float64
		frames      [2]int // 10 ms, 2.5 ms
		voiced      [][2]float64
		silent      [][2]float64
		hz          float64
		golden      [][2]float64 // seconds, Hz
	}{
		{
			name: "silence", rate: 16000, count: 16080, channels: 1,
			signal: func(float64) float64 { return 0 },
			frames: [2]int{101, 403}, silent: [][2]float64{{0, 1.005}},
		},
		{
			// Harvest need not classify a pure sinusoid as voiced.
			name: "pure_tone", rate: 16000, count: 16080, channels: 1,
			signal: func(seconds float64) float64 { return .3 * math.Sin(2*math.Pi*220*seconds) },
			frames: [2]int{101, 403},
		},
		{
			name: "harmonics", rate: 16000, count: 16080, channels: 1,
			signal: func(seconds float64) float64 { return harmonics(200, seconds) },
			frames: [2]int{101, 403}, voiced: [][2]float64{{.2, .8}}, hz: 200,
			golden: [][2]float64{{.3, 200.007338119}, {.5, 200.007622497}, {.8, 200.007162464}},
		},
		{
			name: "stereo_harmonics", rate: 44100, count: 44320, channels: 2,
			signal: func(seconds float64) float64 { return harmonics(330, seconds) },
			frames: [2]int{101, 402}, voiced: [][2]float64{{.2, .8}}, hz: 330,
			golden: [][2]float64{{.5, 329.986760451}},
		},
		{
			name: "stereo_cancellation", rate: 16000, count: 16080, channels: 2, antiPhase: true,
			signal: func(seconds float64) float64 { return harmonics(200, seconds) },
			frames: [2]int{101, 403}, silent: [][2]float64{{0, 1.005}},
		},
		{
			name: "pause", rate: 16000, count: 32000, channels: 1,
			signal: func(seconds float64) float64 {
				if seconds < .1 || seconds >= .7 && seconds < 1.3 || seconds >= 1.9 {
					return 0
				}
				return harmonics(200, seconds)
			},
			frames: [2]int{201, 801}, voiced: [][2]float64{{.25, .55}, {1.45, 1.75}},
			silent: [][2]float64{{.85, 1.15}}, hz: 200,
			golden: [][2]float64{{.3, 200.007500603}, {1.5, 200.007310468}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pcm := &wav.PCM{SampleRate: tc.rate, Channels: tc.channels, Data: make([]int16, tc.count*tc.channels)}
			mono := make([]float64, tc.count)
			for i := range mono {
				sample := int16(math.Round(32767 * tc.signal(float64(i)/float64(tc.rate))))
				for channel := 0; channel < tc.channels; channel++ {
					value := sample
					if channel == 1 && tc.antiPhase {
						value = -value
					}
					pcm.Data[i*tc.channels+channel] = value
					mono[i] += float64(value) / 32768 / float64(tc.channels)
				}
			}
			path := filepath.Join(t.TempDir(), "input.wav")
			if err := wavfile.Write(path, pcm); err != nil {
				t.Fatal(err)
			}
			for stepIndex, step := range []float64{10, 2.5} {
				t.Run(fmt.Sprintf("%gms", step), func(t *testing.T) {
					got, err := WorldF0(path, step, enginePath)
					if err != nil {
						t.Fatal(err)
					}
					want, err := world.F0(mono, tc.rate, step)
					if err != nil {
						t.Fatal(err)
					}
					// WORLD GetSamplesForHarvest: floor(1000 * samples / rate / step) + 1.
					frames := tc.frames[stepIndex]
					if len(got) != frames || len(want) != frames {
						t.Fatalf("frames: file=%d reference=%d, want %d", len(got), len(want), frames)
					}
					for i, f0 := range got {
						if math.IsNaN(f0) || math.IsInf(f0, 0) || f0 < 0 {
							t.Fatalf("frame %d: invalid F0 %g", i, f0)
						}
						if math.IsNaN(want[i]) || math.IsInf(want[i], 0) || math.Abs(f0-want[i]) > 1e-6 {
							t.Fatalf("frame %d: file F0 %.9f, reference %.9f", i, f0, want[i])
						}
						seconds := float64(i) * step / 1000
						for _, span := range tc.voiced {
							if seconds >= span[0] && seconds <= span[1] && math.Abs(f0-tc.hz) > .5 {
								t.Errorf("frame %d (%gs): F0 %g, want %g +/- 0.5 Hz", i, seconds, f0, tc.hz)
							}
						}
						for _, span := range tc.silent {
							if seconds >= span[0] && seconds <= span[1] && f0 != 0 {
								t.Errorf("frame %d (%gs): silent F0 %g, want 0", i, seconds, f0)
							}
						}
					}
					// Frozen with UtauWLD v0.1.2 / WORLD d625e760. Only interior frames
					// are pinned; 0.05 Hz allows numerical variation between native builds.
					for _, point := range tc.golden {
						frame := int(math.Round(point[0] * 1000 / step))
						if math.Abs(got[frame]-point[1]) > .05 {
							t.Errorf("frame %d: F0 %.9f, golden %.9f +/- 0.05 Hz", frame, got[frame], point[1])
						}
					}
				})
			}
		})
	}
}
