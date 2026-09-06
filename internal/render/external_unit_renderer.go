package render

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/plan"
	"utautts/internal/provider"
)

const unitRendererJobVersion = provider.UnitRendererJobVersion

// Keep the old local names as aliases while all unit-renderer adapters use
// the shared contract types from internal/provider.
type externalUnitRendererJob = provider.UnitRendererJob
type externalUnitRendererJobOptions = provider.UnitRendererOptions

type externalUnitRenderer struct {
	definition engine.Definition
}

func newExternalUnitRenderer(definition engine.Definition) UnitRenderer {
	return externalUnitRenderer{definition: definition}
}

func (renderer externalUnitRenderer) ProviderID() engine.ProviderID {
	return renderer.definition.Provider
}

func (renderer externalUnitRenderer) Render(synthesisPlan *plan.Plan, cfg Config) (*UnitRenderResult, error) {
	if synthesisPlan == nil || len(synthesisPlan.Units) == 0 {
		return nil, errors.New("empty synthesis plan")
	}
	if err := contextError(cfg.Context); err != nil {
		return nil, err
	}
	executable := strings.TrimSpace(renderer.definition.Resource(engine.ResourceProviderExecutable))
	if executable == "" {
		return nil, errors.New("external provider executable is not configured")
	}
	jobDirectory, err := os.MkdirTemp("", "utautts-provider-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(jobDirectory)

	planData, err := json.Marshal(plan.Clone(synthesisPlan))
	if err != nil {
		return nil, fmt.Errorf("encode external unit renderer plan: %w", err)
	}
	job := externalUnitRendererJob{
		Version:         unitRendererJobVersion,
		Contract:        string(renderer.definition.Contract),
		ContractVersion: renderer.definition.ContractVersion,
		Plan:            planData,
		Options: externalUnitRendererJobOptions{
			ReleaseMS:               cfg.ReleaseMS,
			LeadingPreutteranceMS:   cfg.LeadingPreutteranceMS,
			IntonationStrength:      cfg.IntonationStrength,
			ApplyPitch:              cfg.ApplyPitch,
			BoundaryBridgeMS:        cfg.BoundaryBridgeMS,
			BoundaryBridgeThreshold: cfg.BoundaryBridgeThreshold,
			CVVCTiming:              cfg.CVVCTiming,
			CVVCTransitionGain:      cfg.CVVCTransitionGain,
			CVVCPreBoundaryFade:     cfg.CVVCPreBoundaryFade,
			PitchCurve:              providerPitchCurve(cfg.PitchCurve),
		},
		Resources: definitionResources(renderer.definition),
	}
	inputPath := filepath.Join(jobDirectory, "input.json")
	outputPath := filepath.Join(jobDirectory, "output.wav")
	data, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("encode external unit renderer job: %w", err)
	}
	if err := os.WriteFile(inputPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write external unit renderer job: %w", err)
	}

	session, key, err := externalProviderSessions.get(cfg.Context, renderer.definition)
	if err != nil {
		return nil, err
	}
	var diagnostics []RenderDiagnostic
	result, err := session.Render(cfg.Context, provider.RenderRequest{
		Contract:        string(renderer.definition.Contract),
		ContractVersion: renderer.definition.ContractVersion,
		InputPath:       inputPath,
		OutputPath:      outputPath,
	}, provider.RenderOptions{OnDiagnostic: func(value provider.Diagnostic) {
		diagnostics = append(diagnostics, RenderDiagnostic{Severity: value.Severity, Code: value.Code, Message: value.Message})
	}})
	if err != nil {
		if !session.IsAlive() {
			externalProviderSessions.discard(key, session)
		}
		return nil, fmt.Errorf("external unit renderer: %w", err)
	}
	artifactPath := strings.TrimSpace(result.Audio.Path)
	if artifactPath == "" {
		artifactPath = outputPath
	} else if !filepath.IsAbs(artifactPath) {
		artifactPath = filepath.Join(jobDirectory, artifactPath)
	}
	if err := ensureJobPath(jobDirectory, artifactPath); err != nil {
		return nil, fmt.Errorf("external unit renderer output: %w", err)
	}
	pcm, err := audio.ReadWav(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("read external unit renderer output: %w", err)
	}
	report := RenderReport{Provider: renderer.definition.Provider, Diagnostics: diagnostics}
	if len(result.Report) > 0 {
		if reportData, marshalErr := json.Marshal(result.Report); marshalErr == nil {
			var providerReport RenderReport
			if json.Unmarshal(reportData, &providerReport) == nil {
				report = providerReport
				if report.Provider == "" {
					report.Provider = renderer.definition.Provider
				}
				report.Diagnostics = append(diagnostics, report.Diagnostics...)
			}
		}
	}
	return &UnitRenderResult{Audio: pcm, Report: report}, nil
}

func definitionResources(definition engine.Definition) map[string]string {
	if len(definition.Resources) == 0 {
		return nil
	}
	result := make(map[string]string, len(definition.Resources))
	for key, value := range definition.Resources {
		result[string(key)] = value
	}
	return result
}

func providerPitchCurve(curve *PitchCurve) *provider.PitchCurve {
	if curve == nil {
		return nil
	}
	return &provider.PitchCurve{FrameMS: curve.FrameMS, Cents: append([]float64(nil), curve.Cents...)}
}

func ensureJobPath(directory, path string) error {
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

type externalProviderSessionPool struct {
	mu       sync.Mutex
	sessions map[string]*provider.Session
}

func (pool *externalProviderSessionPool) closeAll() error {
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

var externalProviderSessions = externalProviderSessionPool{sessions: make(map[string]*provider.Session)}

func (pool *externalProviderSessionPool) get(ctx context.Context, definition engine.Definition) (*provider.Session, string, error) {
	executable := strings.TrimSpace(definition.Resource(engine.ResourceProviderExecutable))
	if executable == "" {
		return nil, "", errors.New("external provider executable is not configured")
	}
	key := externalProviderSessionKey(definition, executable)
	pool.mu.Lock()
	defer pool.mu.Unlock()
	if session := pool.sessions[key]; session != nil && session.IsAlive() {
		return session, key, nil
	}
	if old := pool.sessions[key]; old != nil {
		_ = old.Close()
		delete(pool.sessions, key)
	}
	protocolVersion := definition.ProtocolVersion
	if protocolVersion == 0 {
		protocolVersion = provider.ProtocolVersion
	}
	contractVersion := definition.ContractVersion
	if contractVersion <= 0 {
		return nil, "", errors.New("external provider contract version must be positive")
	}
	session, err := provider.StartSession(ctx, provider.SessionOptions{
		Executable:      executable,
		Args:            append([]string(nil), definition.ProviderArgs...),
		Dir:             filepath.Dir(executable),
		Provider:        string(definition.Provider),
		ProviderVersion: definition.ProviderVersion,
		Capabilities:    providerCapabilities(definition.Capabilities),
		Contract:        string(definition.Contract),
		ContractVersion: contractVersion,
		ProtocolVersion: protocolVersion,
	})
	if err != nil {
		return nil, "", err
	}
	pool.sessions[key] = session
	return session, key, nil
}

func providerCapabilities(capabilities engine.Capabilities) []string {
	// The host always sends the shared unit-renderer job envelope. Feature
	// capabilities below describe optional behavior inside that envelope; the
	// envelope capability itself is therefore required for every external unit
	// renderer session.
	result := []string{provider.CapabilityUnitRendererJobV1}
	if capabilities.FramePitch {
		result = append(result, "frame_pitch")
	}
	if capabilities.BoundaryBridge {
		result = append(result, "boundary_bridge")
	}
	return result
}

func externalProviderSessionKey(definition engine.Definition, executable string) string {
	values := []string{
		executable,
		string(definition.Provider),
		definition.ProviderVersion,
		string(definition.Contract),
		fmt.Sprint(definition.ContractVersion),
		fmt.Sprint(definition.ProtocolVersion),
	}
	values = append(values, definition.ProviderArgs...)
	resourceKeys := make([]string, 0, len(definition.Resources))
	for key := range definition.Resources {
		resourceKeys = append(resourceKeys, string(key))
	}
	sort.Strings(resourceKeys)
	for _, key := range resourceKeys {
		values = append(values, key, definition.Resources[engine.ResourceKey(key)])
	}
	return strings.Join(values, "\x00")
}

func (pool *externalProviderSessionPool) discard(key string, session *provider.Session) {
	pool.mu.Lock()
	if pool.sessions[key] == session {
		delete(pool.sessions, key)
	}
	pool.mu.Unlock()
	_ = session.Close()
}
