package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
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
	if len(args) == 1 && args[0] == "--serve" {
		return serve(os.Stdin, os.Stdout)
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: utautts-worldline-bridge MANIFEST.json | --serve")
	}
	return renderManifest(args[0], nil)
}

type serveResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func serve(input *os.File, output *os.File) error {
	state := &bridgeState{
		libraries: make(map[string]nativeLibrary), worldEngines: make(map[string]worldEngine),
		worldUnits: newWorldFeatureCache(128),
	}
	defer func() {
		for _, library := range state.libraries {
			_ = library.Close()
		}
		for _, engine := range state.worldEngines {
			_ = engine.Close()
		}
	}()
	scanner := bufio.NewScanner(input)
	writer := bufio.NewWriter(output)
	for scanner.Scan() {
		path := scanner.Text()
		err := renderManifest(path, state)
		response := serveResponse{OK: err == nil}
		if err != nil {
			response.Error = err.Error()
		}
		data, _ := json.Marshal(response)
		if _, writeErr := writer.Write(append(data, '\n')); writeErr != nil {
			return writeErr
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

func renderManifest(path string, state *bridgeState) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var input manifest
	if err := json.Unmarshal(data, &input); err != nil {
		return fmt.Errorf("decode manifest: %w", err)
	}
	if len(input.Units) == 0 || len(input.F0Curve) < 2 {
		return fmt.Errorf("manifest has no synthesis data")
	}
	if input.Engine == "utautts-world-phrase" || input.Engine == "utautts-world-phrase-cuda" {
		var engine worldEngine
		if state != nil {
			engine = state.worldEngines[input.WorldEnginePath]
		}
		if engine == nil {
			engine, err = openWorldEngine(input.WorldEnginePath)
			if err != nil {
				return err
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
			return renderErr
		}
		return writePCM16(input.OutputPath, input.SampleRate, samples)
	}
	var library nativeLibrary
	if state != nil {
		library = state.libraries[input.WorldlinePath]
	}
	if library == nil {
		library, err = openNativeLibrary(input.WorldlinePath)
		if err != nil {
			return err
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
		return err
	}
	return writePCM16(input.OutputPath, input.SampleRate, samples)
}
