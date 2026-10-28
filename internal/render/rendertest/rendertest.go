// Package rendertestは、WORLDのライブラリなしで合成の流れを確かめるテスト用の描画を登録する。
// テストでimportすると、既定の描画がこれに替わる。原音を音程も長さも変えずに並べる。
package rendertest

import (
	"errors"
	"fmt"
	"math"

	"github.com/yh2237/UtauIDX/audio"
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/UtauTTS/internal/render/base"
)

func init() {
	base.TestRenderer = renderConcat
}

func renderConcat(synthesisPlan *plan.Plan, cfg base.Config) (*audio.PCM, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	cache := base.NewSourceCache()
	sampleRate := 0
	var mix []float64
	for _, unit := range synthesisPlan.Units {
		if err := base.ContextError(cfg.Context); err != nil {
			return nil, err
		}
		if unit.Silent {
			continue
		}
		mono, err := cache.LoadMono(unit.Source)
		if err != nil {
			return nil, fmt.Errorf("read unit %q (%s): %w", unit.Alias, unit.Source, err)
		}
		if sampleRate == 0 {
			sampleRate = mono.SampleRate
		}
		if mono.SampleRate != sampleRate {
			if mono, err = cache.LoadNormalized(unit.Source, sampleRate); err != nil {
				return nil, fmt.Errorf("normalize unit %q: %w", unit.Alias, err)
			}
		}
		trimmed, err := audio.TrimPCM(mono, unit.OffsetMS, unit.CutoffMS)
		if err != nil {
			return nil, fmt.Errorf("trim unit %q: %w", unit.Alias, err)
		}
		wave := base.PcmFloats(trimmed.Data)
		start := max(0, base.MsToFramesSigned(unit.NoteStartMS-unit.PreutteranceMS, sampleRate))
		length := min(len(wave), base.MsToFrames(unit.PreutteranceMS+unit.DurationMS, sampleRate))
		if end := start + length; end > len(mix) {
			mix = append(mix, make([]float64, end-len(mix))...)
		}
		for frame := range length {
			mix[start+frame] += wave[frame]
		}
	}
	if sampleRate == 0 {
		return nil, errors.New("render produced no samples")
	}
	if minimum := base.MsToFrames(synthesisPlan.DurationMS, sampleRate); len(mix) < minimum {
		mix = append(mix, make([]float64, minimum-len(mix))...)
	}
	for frame, value := range mix {
		mix[frame] = math.Max(-1, math.Min(1, value))
	}
	return &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: base.FloatPCM(mix)}, nil
}
