package worldline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"utautts/internal/provider"
)

type BridgeJob struct {
	Anchors     bool
	CodaRelease bool
	Speech      bool
	Engine      string `json:"engine"`
}

func invokeBridge(ctx context.Context, bridge, jobPath, outputPath string) error {
	return InvokeReport(ctx, bridge, jobPath, outputPath, nil)
}

func ReadBridgeJob(path string) (BridgeJob, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return BridgeJob{}, fmt.Errorf("read worldline job: %w", err)
	}
	var commonJob provider.UnitRendererJob
	if err := json.Unmarshal(data, &commonJob); err != nil {
		return BridgeJob{}, fmt.Errorf("decode worldline job: %w", err)
	}
	if commonJob.Version != provider.UnitRendererJobVersion ||
		commonJob.Contract != "unit-renderer" || commonJob.ContractVersion != 1 {
		return BridgeJob{}, fmt.Errorf("unsupported worldline job contract")
	}
	if commonJob.Options.Worldline == nil {
		return BridgeJob{}, fmt.Errorf("worldline job has no typed worldline options")
	}
	job := BridgeJob{Engine: commonJob.Options.Worldline.Engine}
	for _, unit := range commonJob.Options.Worldline.Units {
		job.Speech = job.Speech || unit.Speech != nil
		job.CodaRelease = job.CodaRelease || unit.Speech != nil && unit.Speech.CodaRelease
		job.Anchors = job.Anchors || unit.Speech != nil && len(unit.Speech.Anchors) > 0
	}
	return validateBridgeJob(job)
}

func validateBridgeJob(job BridgeJob) (BridgeJob, error) {
	if job.Engine == "" {
		return BridgeJob{}, fmt.Errorf("worldline job has no engine")
	}
	return job, nil
}

func providerIDForEngine(engineID string) (string, error) {
	switch engineID {
	case "utautts-world-phrase":
		return engineID, nil
	default:
		return "", fmt.Errorf("unknown worldline bridge engine %q", engineID)
	}
}
