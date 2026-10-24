package worldrender

import (
	"context"
	"fmt"

	"github.com/yh2237/UtauTTS/internal/audio"
)

type phraseInput struct {
	SpeechResults   *[]SpeechResult `json:"-"`
	WorldEnginePath string          `json:"world_engine_path"`
	SampleRate      int             `json:"sample_rate"`
	F0Curve         []float64       `json:"f0_curve"`
	Units           []Unit          `json:"units"`
	timingWarp      *timingWarp
}

type Unit struct {
	Speech            *SpeechTiming   `json:"speech,omitempty"`
	LegacyMix         bool            `json:"legacy_mix,omitempty"`
	GapRepair         bool            `json:"gap_repair,omitempty"`
	CacheKey          string          `json:"cache_key"`
	Source            string          `json:"source"`
	FRQPath           string          `json:"frq_path"`
	PositionMS        float64         `json:"position_ms"`
	SkipMS            float64         `json:"skip_ms"`
	LengthMS          float64         `json:"length_ms"`
	FadeInMS          float64         `json:"fade_in_ms"`
	FadeOutMS         float64         `json:"fade_out_ms"`
	OffsetMS          float64         `json:"offset_ms"`
	RequiredLengthMS  float64         `json:"required_length_ms"`
	ConsonantMS       float64         `json:"consonant_ms"`
	CutoffMS          float64         `json:"cutoff_ms"`
	ConsonantVelocity float64         `json:"consonant_velocity"`
	Volume            float64         `json:"volume"`
	VolumeSet         bool            `json:"volume_set"`
	EnergyFactor      float64         `json:"energy_factor"`
	Envelope          []EnvelopePoint `json:"envelope"`
}

type EnvelopePoint struct {
	XMS float64 `json:"x_ms"`
	Y   float64 `json:"y"`
}

func newRendererState() *rendererState {
	return &rendererState{
		worldEngines: make(map[string]worldEngine),
		worldUnits:   newWorldFeatureCache(128),
	}
}

// Rendererは読み込んだWORLDエンジンと分析結果を持ち続ける。同時には呼ばない。
type Renderer struct {
	state *rendererState
}

func NewRenderer() *Renderer {
	return &Renderer{state: newRendererState()}
}

// Renderは句をWORLDで描画する。enginePathはWORLDのライブラリ。
func (renderer *Renderer) Render(ctx context.Context, phrase Phrase, enginePath string) (*audio.PCM, []SpeechResult, error) {
	input := phraseInput{
		WorldEnginePath: enginePath, SampleRate: phrase.SampleRate,
		F0Curve: phrase.F0Curve, Units: phrase.Units, timingWarp: timingWarpFromJob(phrase.TimingWarp),
	}
	samples, results, err := renderManifest(ctx, input, renderer.state)
	if err != nil {
		return nil, nil, err
	}
	return &audio.PCM{SampleRate: input.SampleRate, Channels: 1, Data: pcm16(samples)}, results, nil
}

type rendererState struct {
	worldEngines map[string]worldEngine
	worldUnits   *worldFeatureCache
}

func renderManifest(ctx context.Context, input phraseInput, state *rendererState) ([]float32, []SpeechResult, error) {
	input.SpeechResults = new([]SpeechResult)
	var err error
	if len(input.Units) == 0 || len(input.F0Curve) < 2 {
		return nil, nil, fmt.Errorf("manifest has no synthesis data")
	}
	var engine worldEngine
	if state != nil {
		engine = state.worldEngines[input.WorldEnginePath]
	}
	if engine == nil {
		engine, err = openWorldEngine(input.WorldEnginePath)
		if err != nil {
			return nil, nil, err
		}
		if state != nil {
			// 別のパスのライブラリは使わなくなったので解放する。
			for path, previous := range state.worldEngines {
				_ = previous.Close()
				delete(state.worldEngines, path)
			}
			state.worldEngines[input.WorldEnginePath] = engine
		} else {
			defer engine.Close()
		}
	}
	var cache *worldFeatureCache
	if state != nil {
		cache = state.worldUnits
	}
	samples, err := renderUtauTTSWorldPhrase(ctx, engine, input, cache)
	if err != nil {
		return nil, nil, err
	}
	return samples, *input.SpeechResults, nil
}
