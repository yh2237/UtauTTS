package worldrender

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yh2237/UtauTTS/internal/provider"
)

type manifest struct {
	SpeechResults   *[]provider.WorldSpeechResult `json:"-"`
	Engine          string                        `json:"engine"`
	WorldEnginePath string                        `json:"world_engine_path"`
	OutputPath      string                        `json:"output_path"`
	SampleRate      int                           `json:"sample_rate"`
	F0Curve         []float64                     `json:"f0_curve"`
	Units           []unit                        `json:"units"`
	timingWarp      *timingWarp
}

type unit struct {
	Speech            *provider.WorldSpeechTiming `json:"speech,omitempty"`
	LegacyMix         bool                        `json:"legacy_mix,omitempty"`
	GapRepair         bool                        `json:"gap_repair,omitempty"`
	CacheKey          string                      `json:"cache_key"`
	Source            string                      `json:"source"`
	FrqPath           string                      `json:"frq_path"`
	PositionMS        float64                     `json:"position_ms"`
	SkipMS            float64                     `json:"skip_ms"`
	LengthMS          float64                     `json:"length_ms"`
	FadeInMS          float64                     `json:"fade_in_ms"`
	FadeOutMS         float64                     `json:"fade_out_ms"`
	OffsetMS          float64                     `json:"offset_ms"`
	RequiredLengthMS  float64                     `json:"required_length_ms"`
	ConsonantMS       float64                     `json:"consonant_ms"`
	CutoffMS          float64                     `json:"cutoff_ms"`
	ConsonantVelocity float64                     `json:"consonant_velocity"`
	Volume            float64                     `json:"volume"`
	VolumeSet         bool                        `json:"volume_set"`
	EnergyFactor      float64                     `json:"energy_factor"`
	Envelope          []envelopePoint             `json:"envelope"`
}

type envelopePoint struct {
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

func (renderer *Renderer) RenderJob(ctx context.Context, jobJSON []byte, outputPath string) ([]provider.WorldSpeechResult, error) {
	input, err := decodeProviderJob(jobJSON, outputPath)
	if err != nil {
		return nil, err
	}
	result, err := renderManifestValue(ctx, input, outputPath, renderer.state)
	if err != nil {
		return nil, err
	}
	if result.SpeechResults != nil {
		return *result.SpeechResults, nil
	}
	return nil, nil
}

type rendererState struct {
	worldEngines map[string]worldEngine
	worldUnits   *worldFeatureCache
}

func decodeProviderJob(data []byte, outputPath string) (manifest, error) {
	var job provider.UnitRendererJob
	if err := json.Unmarshal(data, &job); err != nil {
		return manifest{}, fmt.Errorf("decode worldline job: %w", err)
	}
	if job.Version != provider.UnitRendererJobVersion || job.Contract != "unit-renderer" || job.ContractVersion != 1 {
		return manifest{}, fmt.Errorf("unsupported worldline job contract")
	}
	if job.Options.Worldline == nil {
		return manifest{}, fmt.Errorf("worldline job has no typed worldline options")
	}
	options := job.Options.Worldline
	if options.Engine != "utautts-world-phrase" {
		return manifest{}, fmt.Errorf("unsupported worldline engine %q", options.Engine)
	}
	input := manifest{
		Engine: options.Engine, OutputPath: outputPath, SampleRate: options.SampleRate,
		F0Curve: append([]float64(nil), options.F0Curve...), Units: make([]unit, len(options.Units)),
		WorldEnginePath: job.Resources["world_engine"],
	}
	input.timingWarp = timingWarpFromJob(options.TimingWarp)
	for index, source := range options.Units {
		target := unit{
			Speech: source.Speech, LegacyMix: source.LegacyMix, GapRepair: source.GapRepair,
			CacheKey: source.CacheKey, Source: source.Source, FrqPath: source.FRQPath,
			PositionMS: source.PositionMS, SkipMS: source.SkipMS, LengthMS: source.LengthMS,
			FadeInMS: source.FadeInMS, FadeOutMS: source.FadeOutMS, OffsetMS: source.OffsetMS,
			RequiredLengthMS: source.RequiredLengthMS, ConsonantMS: source.ConsonantMS,
			CutoffMS: source.CutoffMS, ConsonantVelocity: source.ConsonantVelocity,
			Volume: source.Volume, VolumeSet: source.VolumeSet, EnergyFactor: source.EnergyFactor,
			Envelope: make([]envelopePoint, len(source.Envelope)),
		}
		for pointIndex, point := range source.Envelope {
			target.Envelope[pointIndex] = envelopePoint{XMS: point.XMS, Y: point.Y}
		}
		input.Units[index] = target
	}
	return input, nil
}

func renderManifestValue(ctx context.Context, input manifest, outputPath string, state *rendererState) (manifest, error) {
	input.SpeechResults = new([]provider.WorldSpeechResult)
	var err error
	if outputPath != "" {
		input.OutputPath = outputPath
	}
	if len(input.Units) == 0 || len(input.F0Curve) < 2 {
		return manifest{}, fmt.Errorf("manifest has no synthesis data")
	}
	if input.Engine != "utautts-world-phrase" {
		return manifest{}, fmt.Errorf("unknown engine: %s", input.Engine)
	}
	var engine worldEngine
	if state != nil {
		engine = state.worldEngines[input.WorldEnginePath]
	}
	if engine == nil {
		engine, err = openWorldEngine(input.WorldEnginePath)
		if err != nil {
			return manifest{}, err
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
	samples, renderErr := renderUtauTTSWorldPhrase(ctx, engine, input, cache)
	if renderErr != nil {
		return manifest{}, renderErr
	}
	return input, writePCM16(input.OutputPath, input.SampleRate, samples)
}
