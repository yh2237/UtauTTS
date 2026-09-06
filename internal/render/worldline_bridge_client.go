package render

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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

func invokeWorldlineBridge(ctx context.Context, bridge, jobPath, outputPath string) error {
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

	job, err := readWorldlineBridgeJob(jobPath)
	if err != nil {
		return err
	}
	providerID, err := worldlineProviderID(job.Engine)
	if err != nil {
		return err
	}

	client := &sharedWorldlineBridge
	if client.session == nil || client.path != bridge || client.provider != providerID || !client.session.IsAlive() {
		client.stop()
		session, startErr := provider.StartSession(ctx, provider.SessionOptions{
			Executable:      bridge,
			Args:            []string{"--provider", providerID},
			Provider:        providerID,
			ProviderVersion: "1",
			Capabilities:    []string{provider.CapabilityUnitRendererJobV2},
			Contract:        "unit-renderer",
			ContractVersion: 1,
			ProtocolVersion: provider.ProtocolVersion,
		})
		if startErr != nil {
			return fmt.Errorf("start worldline provider session: %w", startErr)
		}
		client.path = bridge
		client.provider = providerID
		client.session = session
	}

	_, err = client.session.Render(ctx, provider.RenderRequest{
		Contract:        "unit-renderer",
		ContractVersion: 1,
		InputPath:       jobPath,
		OutputPath:      outputPath,
	}, provider.RenderOptions{})
	if err != nil && !client.session.IsAlive() {
		client.stop()
	}
	return err
}

type worldlineBridgeJob struct {
	Engine string `json:"engine"`
}

func readWorldlineBridgeJob(path string) (worldlineBridgeJob, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return worldlineBridgeJob{}, fmt.Errorf("read worldline job: %w", err)
	}
	var commonJob provider.UnitRendererJob
	if err := json.Unmarshal(data, &commonJob); err != nil {
		return worldlineBridgeJob{}, fmt.Errorf("decode worldline job: %w", err)
	}
	if commonJob.Version != provider.UnitRendererJobVersion ||
		commonJob.Contract != "unit-renderer" || commonJob.ContractVersion != 1 {
		return worldlineBridgeJob{}, fmt.Errorf("unsupported worldline job contract")
	}
	if commonJob.Options.Worldline == nil {
		return worldlineBridgeJob{}, fmt.Errorf("worldline job has no typed worldline options")
	}
	return validateWorldlineBridgeJob(worldlineBridgeJob{Engine: commonJob.Options.Worldline.Engine})
}

func validateWorldlineBridgeJob(job worldlineBridgeJob) (worldlineBridgeJob, error) {
	if job.Engine == "" {
		return worldlineBridgeJob{}, fmt.Errorf("worldline job has no engine")
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

func (client *worldlineBridgeProcess) stop() {
	if client.session != nil {
		_ = client.session.Close()
	}
	client.path, client.provider, client.session = "", "", nil
}
