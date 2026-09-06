package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"utautts/internal/provider"
)

type manifest struct {
	Engine          string    `json:"engine"`
	WorldlinePath   string    `json:"worldline_path"`
	WorldEnginePath string    `json:"world_engine_path"`
	GPUPath         string    `json:"gpu_path"`
	OutputPath      string    `json:"output_path"`
	SampleRate      int       `json:"sample_rate"`
	F0Curve         []float64 `json:"f0_curve"`
	Units           []unit    `json:"units"`
}

type unit struct {
	CacheKey          string          `json:"cache_key"`
	Source            string          `json:"source"`
	FrqPath           string          `json:"frq_path"`
	PositionMS        float64         `json:"position_ms"`
	SkipMS            float64         `json:"skip_ms"`
	LengthMS          float64         `json:"length_ms"`
	FadeInMS          float64         `json:"fade_in_ms"`
	FadeOutMS         float64         `json:"fade_out_ms"`
	OffsetMS          float64         `json:"offset_ms"`
	RequiredLengthMS  float64         `json:"required_length_ms"`
	ConsonantMS       float64         `json:"consonant_ms"`
	CutoffMS          float64         `json:"cutoff_ms"`
	Tone              int             `json:"tone"`
	ConsonantVelocity float64         `json:"consonant_velocity"`
	PitchStartMS      float64         `json:"pitch_start_ms"`
	PitchLengthMS     float64         `json:"pitch_length_ms"`
	Volume            float64         `json:"volume"`
	Modulation        float64         `json:"modulation"`
	Tempo             float64         `json:"tempo"`
	Envelope          []envelopePoint `json:"envelope"`
}

type envelopePoint struct {
	XMS float64 `json:"x_ms"`
	Y   float64 `json:"y"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 2 && args[0] == "--provider" {
		if strings.TrimSpace(args[1]) == "" {
			return fmt.Errorf("provider id must not be empty")
		}
		return serveProvider(os.Stdin, os.Stdout, args[1])
	}
	return fmt.Errorf("usage: utautts-worldline-bridge --provider PROVIDER_ID")
}

func newBridgeState() *bridgeState {
	return &bridgeState{
		libraries: make(map[string]nativeLibrary), worldEngines: make(map[string]worldEngine),
		worldUnits: newWorldFeatureCache(128),
	}
}

func (state *bridgeState) close() {
	for _, library := range state.libraries {
		_ = library.Close()
	}
	for _, engine := range state.worldEngines {
		_ = engine.Close()
	}
}

// serveProvider is the v1 external-provider protocol adapter. It accepts only
// the common unit-renderer job and its typed WORLD options.
func serveProvider(input io.Reader, output io.Writer, providerID string) error {
	state := newBridgeState()
	defer state.close()
	writer := bufio.NewWriter(output)
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(provider.Hello{
		Type: provider.MessageHello, Protocol: provider.ProtocolName, ProtocolVersion: provider.ProtocolVersion,
		Provider: providerID, ProviderVersion: "1", Session: true,
		Capabilities: []string{"frame_pitch", provider.CapabilityUnitRendererJobV2},
		Contracts:    []provider.ContractSupport{{Name: "unit-renderer", Version: 1}},
	}); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		var header struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &header); err != nil {
			if writeErr := encoder.Encode(provider.ErrorMessage{Type: provider.MessageError, Code: "invalid_request", Message: err.Error()}); writeErr != nil {
				return writeErr
			}
			if err := writer.Flush(); err != nil {
				return err
			}
			continue
		}
		if header.Type == provider.MessageShutdown {
			return nil
		}
		if header.Type == provider.MessageCancel {
			// Rendering is synchronous in the native adapter. The host will
			// terminate the process after the protocol cancel grace period if
			// the active render does not finish.
			continue
		}
		var request provider.RenderRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			if writeErr := encoder.Encode(provider.ErrorMessage{Type: provider.MessageError, Code: "invalid_request", Message: err.Error()}); writeErr != nil {
				return writeErr
			}
			if err := writer.Flush(); err != nil {
				return err
			}
			continue
		}
		if request.Type != provider.MessageRender || request.Contract != "unit-renderer" || request.ContractVersion != 1 {
			if err := encoder.Encode(provider.ErrorMessage{Type: provider.MessageError, RequestID: request.RequestID, Code: "unsupported_request", Message: "worldline bridge accepts unit-renderer contract version 1"}); err != nil {
				return err
			}
			if err := writer.Flush(); err != nil {
				return err
			}
			continue
		}
		result, err := renderProviderInputAt(request.InputPath, request.OutputPath, state)
		if err != nil {
			if encodeErr := encoder.Encode(provider.ErrorMessage{Type: provider.MessageError, RequestID: request.RequestID, Code: "render_failed", Message: err.Error()}); encodeErr != nil {
				return encodeErr
			}
		} else if err := encoder.Encode(provider.Result{
			Type: provider.MessageResult, RequestID: request.RequestID,
			Audio: provider.AudioArtifact{Path: result.OutputPath, Format: "wav_pcm_s16le", SampleRate: result.SampleRate, Channels: 1},
		}); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
	}
	return scanner.Err()
}

type bridgeState struct {
	libraries    map[string]nativeLibrary
	worldEngines map[string]worldEngine
	worldUnits   *worldFeatureCache
}

func renderProviderInputAt(path, outputPath string, state *bridgeState) (manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest{}, err
	}
	input, err := decodeProviderJob(data, outputPath)
	if err != nil {
		return manifest{}, err
	}
	return renderManifestValue(input, outputPath, state)
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
	input := manifest{
		Engine: options.Engine, OutputPath: outputPath, SampleRate: options.SampleRate,
		F0Curve: append([]float64(nil), options.F0Curve...), Units: make([]unit, len(options.Units)),
		WorldlinePath: job.Resources["worldline"], WorldEnginePath: job.Resources["world_engine"],
		GPUPath: job.Resources["world_gpu"],
	}
	for index, source := range options.Units {
		target := unit{
			CacheKey: source.CacheKey, Source: source.Source, FrqPath: source.FRQPath,
			PositionMS: source.PositionMS, SkipMS: source.SkipMS, LengthMS: source.LengthMS,
			FadeInMS: source.FadeInMS, FadeOutMS: source.FadeOutMS, OffsetMS: source.OffsetMS,
			RequiredLengthMS: source.RequiredLengthMS, ConsonantMS: source.ConsonantMS,
			CutoffMS: source.CutoffMS, Tone: source.Tone, ConsonantVelocity: source.ConsonantVelocity,
			PitchStartMS: source.PitchStartMS, PitchLengthMS: source.PitchLengthMS,
			Volume: source.Volume, Modulation: source.Modulation, Tempo: source.Tempo,
			Envelope: make([]envelopePoint, len(source.Envelope)),
		}
		for pointIndex, point := range source.Envelope {
			target.Envelope[pointIndex] = envelopePoint{XMS: point.XMS, Y: point.Y}
		}
		input.Units[index] = target
	}
	return input, nil
}

func renderManifestValue(input manifest, outputPath string, state *bridgeState) (manifest, error) {
	var err error
	if outputPath != "" {
		input.OutputPath = outputPath
	}
	if len(input.Units) == 0 || len(input.F0Curve) < 2 {
		return manifest{}, fmt.Errorf("manifest has no synthesis data")
	}
	if input.Engine == "utautts-world-phrase" || input.Engine == "utautts-world-phrase-cuda" {
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
				state.worldEngines[input.WorldEnginePath] = engine
			} else {
				defer engine.Close()
			}
		}
		var cache *worldFeatureCache
		if state != nil {
			cache = state.worldUnits
		}
		samples, renderErr := renderUtauTTSWorldPhrase(engine, input, cache)
		if renderErr != nil {
			return manifest{}, renderErr
		}
		return input, writePCM16(input.OutputPath, input.SampleRate, samples)
	}
	var library nativeLibrary
	if state != nil {
		library = state.libraries[input.WorldlinePath]
	}
	if library == nil {
		library, err = openNativeLibrary(input.WorldlinePath)
		if err != nil {
			return manifest{}, err
		}
		if state != nil {
			state.libraries[input.WorldlinePath] = library
		} else {
			defer library.Close()
		}
	}

	var samples []float32
	switch input.Engine {
	case "worldline-r-faithful":
		samples, err = renderWorldlineR(library, input)
	default:
		err = fmt.Errorf("unknown engine: %s", input.Engine)
	}
	if err != nil {
		return manifest{}, err
	}
	return input, writePCM16(input.OutputPath, input.SampleRate, samples)
}
