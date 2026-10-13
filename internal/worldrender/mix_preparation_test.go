package worldrender

import (
	"math"
	"math/rand/v2"
	"testing"

	"github.com/yh2237/UtauTTS/internal/provider"
)

func TestWorldFrameUnitsKeepsAllActiveUnitsInOrder(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 7))
	input := manifest{F0Curve: make([]float64, 600)}
	for range 200 {
		input.Units = append(input.Units, unit{PositionMS: rng.Float64()*6500 - 500, LengthMS: rng.Float64() * 800})
	}
	input.Units = append(input.Units, unit{PositionMS: 10, LengthMS: 0}, unit{PositionMS: math.NaN(), LengthMS: 100}, unit{LengthMS: math.Inf(1)})
	offsets, units := worldFrameUnits(input)
	for frame := range input.F0Curve {
		candidates := units[offsets[frame]:offsets[frame+1]]
		cursor := 0
		for index, item := range input.Units {
			local := float64(frame)*worldFramePeriodMS - item.PositionMS
			if local < 0 || local > item.LengthMS {
				continue
			}
			for cursor < len(candidates) && candidates[cursor] < index {
				cursor++
			}
			if cursor >= len(candidates) || candidates[cursor] != index {
				t.Fatalf("frame %d lost unit %d", frame, index)
			}
		}
		for i := 1; i < len(candidates); i++ {
			if candidates[i] <= candidates[i-1] {
				t.Fatal("unit order changed")
			}
		}
	}
}

func TestWorldPreparedMappingMatchesOriginal(t *testing.T) {
	entry := cachedWorldUnit{duration: 300, sourceShiftMS: 3.5}
	for _, speech := range []*provider.WorldSpeechTiming{nil,
		{SourceOnsetMS: 50, TargetOnsetMS: 40},
		{CodaRelease: true, SourceOnsetMS: 40, TargetOnsetMS: 30, ProtectStop: true},
		{Anchors: []provider.SpeechAnchor{{SourceMS: 0, TargetMS: 0}, {SourceMS: 90, TargetMS: 60}, {SourceMS: 290, TargetMS: 180}}},
		{Anchors: []provider.SpeechAnchor{{SourceMS: 20, TargetMS: 0}}},
	} {
		for _, legacy := range []bool{false, true} {
			item := unit{Speech: speech, LegacyMix: legacy, OffsetMS: 123.5, ConsonantMS: 70, RequiredLengthMS: 180, LengthMS: 180, ConsonantVelocity: 86}
			mapping, ok := worldSpeechAnchors(item, entry.duration)
			for local := -10.; local <= 210; local += .5 {
				want := mapWorldSourceTime(item, entry.duration, local)
				if !legacy {
					if _, valid := worldSpeechAnchors(item, entry.duration); !valid {
						want += entry.sourceShiftMS
					}
				}
				got := mapWorldFeatureTimePrepared(item, entry, local, mapping, ok)
				if got != want {
					t.Fatalf("mapping changed: %g != %g", got, want)
				}
			}
		}
	}
}

func BenchmarkWorldMixPrepared(b *testing.B) {
	const frames = 600
	input := manifest{F0Curve: make([]float64, frames), Units: make([]unit, 100)}
	for i := range input.Units {
		input.Units[i] = unit{PositionMS: float64(i) * 60, LengthMS: 120, RequiredLengthMS: 120, ConsonantVelocity: 100}
	}
	prepared := flatPreparedUnits(len(input.Units))
	for _, tc := range []struct {
		name string
		run  func(manifest, []preparedWorldUnit, int, int) worldFeatures
	}{{"reference", mixWorldFeaturesReference}, {"prepared", mixWorldFeatures}} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				tc.run(input, prepared, 2, 1)
			}
		})
	}
}
