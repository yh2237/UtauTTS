package diffsinger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/provider"
)

// RenderSession uses the provider protocol when the bridge supports it. The
// bridge process stays alive between calls so ONNX Runtime sessions can remain
// resident. Older bridge binaries are still accepted through the existing
// one-shot Render fallback.
func RenderSession(ctx context.Context, bridgePath string, request Request) (*audio.PCM, error) {
	return renderSession(ctx, bridgePath, engine.NeuralScore{}, request)
}

func renderSession(ctx context.Context, bridgePath string, score engine.NeuralScore, request Request) (*audio.PCM, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	executable, err := diffSingerBridgeExecutable(bridgePath)
	if err != nil {
		return nil, err
	}
	temp, manifestPath, outputPath, err := writeProviderRequest(score, request)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)

	session, key, err := diffSingerProviderSessions.get(ctx, executable)
	if err != nil {
		pcm, fallbackErr := Render(ctx, executable, request)
		if fallbackErr != nil {
			return nil, fmt.Errorf("start DiffSinger provider session: %w (legacy fallback: %v)", err, fallbackErr)
		}
		return pcm, nil
	}
	result, err := session.Render(ctx, provider.RenderRequest{
		Contract:        "neural-synthesizer",
		ContractVersion: 1,
		InputPath:       manifestPath,
		OutputPath:      outputPath,
	}, provider.RenderOptions{})
	if err != nil {
		if !session.IsAlive() {
			diffSingerProviderSessions.discard(key, session)
		}
		return nil, fmt.Errorf("DiffSinger provider session: %w", err)
	}
	artifactPath := strings.TrimSpace(result.Audio.Path)
	if artifactPath == "" {
		artifactPath = outputPath
	} else if !filepath.IsAbs(artifactPath) {
		artifactPath = filepath.Join(temp, artifactPath)
	}
	if err := ensureDiffSingerJobPath(temp, artifactPath); err != nil {
		return nil, fmt.Errorf("DiffSinger provider output: %w", err)
	}
	pcm, err := audio.ReadWav(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("read DiffSinger output: %w", err)
	}
	return pcm, nil
}

func diffSingerBridgeExecutable(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("DiffSinger bridge is not configured")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("DiffSinger bridge %q: %w", path, err)
	}
	return abs, nil
}

func writeProviderRequest(score engine.NeuralScore, request Request) (directory, manifestPath, outputPath string, err error) {
	directory, err = os.MkdirTemp("", "utautts-diffsinger-")
	if err != nil {
		return "", "", "", err
	}
	removeOnError := true
	defer func() {
		if removeOnError {
			_ = os.RemoveAll(directory)
		}
	}()
	request.Version = 1
	outputPath = filepath.Join(directory, "output.wav")
	request.OutputPath = outputPath
	manifestPath = filepath.Join(directory, "request.json")
	options, err := json.Marshal(diffSingerOptions(request))
	if err != nil {
		return "", "", "", err
	}
	data, err := json.Marshal(provider.NeuralSynthesizerJob{
		Version:         provider.NeuralSynthesizerJobVersion,
		Contract:        "neural-synthesizer",
		ContractVersion: 1,
		Score:           score,
		Options:         options,
		Resources:       diffSingerResources(request),
	})
	if err != nil {
		return "", "", "", err
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		return "", "", "", err
	}
	removeOnError = false
	return directory, manifestPath, outputPath, nil
}

func diffSingerOptions(request Request) Request {
	options := request
	options.AcousticPath = ""
	options.VocoderPath = ""
	options.DurationLinguisticPath = ""
	options.DurationPredictorPath = ""
	options.PitchLinguisticPath = ""
	options.PitchPredictorPath = ""
	options.VarianceLinguisticPath = ""
	options.VariancePredictorPath = ""
	return options
}

func diffSingerResources(request Request) map[string]string {
	values := map[string]string{
		"acoustic_model":            request.AcousticPath,
		"vocoder_model":             request.VocoderPath,
		"duration_linguistic_model": request.DurationLinguisticPath,
		"duration_predictor_model":  request.DurationPredictorPath,
		"pitch_linguistic_model":    request.PitchLinguisticPath,
		"pitch_predictor_model":     request.PitchPredictorPath,
		"variance_linguistic_model": request.VarianceLinguisticPath,
		"variance_predictor_model":  request.VariancePredictorPath,
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func ensureDiffSingerJobPath(directory, path string) error {
	root, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	candidate, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path %q is outside the provider job directory", path)
	}
	return nil
}

type diffSingerProviderSessionPool struct {
	mu       sync.Mutex
	sessions map[string]*provider.Session
}

var diffSingerProviderSessions = diffSingerProviderSessionPool{sessions: make(map[string]*provider.Session)}

func (pool *diffSingerProviderSessionPool) get(ctx context.Context, executable string) (*provider.Session, string, error) {
	key := executable
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if session := pool.sessions[key]; session != nil && session.IsAlive() {
		return session, key, nil
	}
	if old := pool.sessions[key]; old != nil {
		_ = old.Close()
		delete(pool.sessions, key)
	}
	session, err := provider.StartSession(ctx, provider.SessionOptions{
		Executable:      executable,
		Args:            []string{"--provider", "diffsinger"},
		Dir:             filepath.Dir(executable),
		Provider:        "diffsinger",
		ProviderVersion: "1",
		Capabilities:    []string{"frame_pitch", provider.CapabilityNeuralScoreJobV1},
		Contract:        "neural-synthesizer",
		ContractVersion: 1,
		ProtocolVersion: provider.ProtocolVersion,
	})
	if err != nil {
		return nil, "", err
	}
	pool.sessions[key] = session
	return session, key, nil
}

func (pool *diffSingerProviderSessionPool) discard(key string, session *provider.Session) {
	pool.mu.Lock()
	if pool.sessions[key] == session {
		delete(pool.sessions, key)
	}
	pool.mu.Unlock()
	_ = session.Close()
}

func (pool *diffSingerProviderSessionPool) closeAll() error {
	pool.mu.Lock()
	sessions := make([]*provider.Session, 0, len(pool.sessions))
	for key, session := range pool.sessions {
		delete(pool.sessions, key)
		sessions = append(sessions, session)
	}
	pool.mu.Unlock()
	var closeErr error
	for _, session := range sessions {
		closeErr = errors.Join(closeErr, session.Close())
	}
	return closeErr
}

// CloseProviderSessions releases the resident DiffSinger bridge process.
func CloseProviderSessions() error {
	return diffSingerProviderSessions.closeAll()
}
