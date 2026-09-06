package render

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"utautts/internal/processutil"
	"utautts/internal/provider"
)

// worldlineBridgeProcess owns the long-lived provider session used by the
// built-in WORLD adapters. The gate keeps the protocol v1 single in-flight
// request guarantee simple while allowing the native library/model state to
// stay resident across syntheses.
type worldlineBridgeProcess struct {
	path     string
	provider string
	session  *provider.Session
}

var sharedWorldlineBridge worldlineBridgeProcess
var worldlineBridgeGate = make(chan struct{}, 1)

func invokeWorldlineBridge(ctx context.Context, bridge, manifestPath string, legacyManifestPath ...string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case worldlineBridgeGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-worldlineBridgeGate }()
	if err := ctx.Err(); err != nil {
		return err
	}

	job, err := readWorldlineBridgeJob(manifestPath)
	if err != nil {
		return err
	}
	providerID, err := worldlineProviderID(job.Engine)
	if err != nil {
		return err
	}

	client := &sharedWorldlineBridge
	legacyPath := manifestPath
	if len(legacyManifestPath) > 0 && strings.TrimSpace(legacyManifestPath[0]) != "" {
		legacyPath = legacyManifestPath[0]
	}
	if client.session == nil || client.path != bridge || client.provider != providerID || !client.session.IsAlive() {
		client.stop()
		session, startErr := provider.StartSession(ctx, provider.SessionOptions{
			Executable:      bridge,
			Args:            []string{"--provider", providerID},
			Provider:        providerID,
			ProviderVersion: "1",
			Capabilities:    []string{provider.CapabilityUnitRendererJobV1},
			Contract:        "unit-renderer",
			ContractVersion: 1,
			ProtocolVersion: provider.ProtocolVersion,
		})
		if startErr != nil {
			// Keep compatibility with an older installed bridge. This path is
			// deliberately only a fallback; the bundled bridge uses the session
			// protocol above and therefore keeps its native runtime resident.
			if fallbackErr := invokeWorldlineBridgeLegacy(ctx, bridge, legacyPath); fallbackErr != nil {
				return fmt.Errorf("start worldline provider session: %w (legacy fallback: %v)", startErr, fallbackErr)
			}
			return nil
		}
		client.path = bridge
		client.provider = providerID
		client.session = session
	}

	_, err = client.session.Render(ctx, provider.RenderRequest{
		Contract:        "unit-renderer",
		ContractVersion: 1,
		InputPath:       manifestPath,
		OutputPath:      job.OutputPath,
	}, provider.RenderOptions{})
	if err != nil && !client.session.IsAlive() {
		client.stop()
	}
	return err
}

type worldlineBridgeJob struct {
	Engine     string `json:"engine"`
	OutputPath string `json:"output_path"`
}

func readWorldlineBridgeJob(path string) (worldlineBridgeJob, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return worldlineBridgeJob{}, fmt.Errorf("read worldline manifest: %w", err)
	}
	var commonJob provider.UnitRendererJob
	if err := json.Unmarshal(data, &commonJob); err == nil &&
		commonJob.Version == provider.UnitRendererJobVersion &&
		commonJob.Contract == "unit-renderer" && commonJob.ContractVersion == 1 &&
		len(commonJob.ProviderPayload) > 0 {
		var payload struct {
			Engine     string `json:"engine"`
			OutputPath string `json:"output_path"`
		}
		if err := json.Unmarshal(commonJob.ProviderPayload, &payload); err != nil {
			return worldlineBridgeJob{}, fmt.Errorf("decode worldline provider payload: %w", err)
		}
		return validateWorldlineBridgeJob(worldlineBridgeJob{Engine: payload.Engine, OutputPath: payload.OutputPath})
	}
	var legacy worldlineBridgeJob
	if err := json.Unmarshal(data, &legacy); err != nil {
		return worldlineBridgeJob{}, fmt.Errorf("decode worldline manifest: %w", err)
	}
	return validateWorldlineBridgeJob(legacy)
}

func validateWorldlineBridgeJob(job worldlineBridgeJob) (worldlineBridgeJob, error) {
	if job.Engine == "" {
		return worldlineBridgeJob{}, fmt.Errorf("worldline manifest has no engine")
	}
	if job.OutputPath == "" {
		return worldlineBridgeJob{}, fmt.Errorf("worldline manifest has no output_path")
	}
	return job, nil
}

func worldlineProviderID(engineID string) (string, error) {
	switch engineID {
	case "worldline-r-faithful":
		return "openutau-worldline-r-faithful", nil
	case "utautts-world-phrase", "utautts-world-phrase-cuda":
		return engineID, nil
	default:
		return "", fmt.Errorf("unknown worldline bridge engine %q", engineID)
	}
}

func invokeWorldlineBridgeLegacy(ctx context.Context, bridge, manifestPath string) error {
	command := exec.CommandContext(ctx, bridge, manifestPath)
	processutil.Configure(command)
	output, err := command.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %s", err, output)
	}
	return nil
}

func (client *worldlineBridgeProcess) stop() {
	if client.session != nil {
		_ = client.session.Close()
	}
	client.path, client.provider, client.session = "", "", nil
}
