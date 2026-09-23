package worldline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"utautts/internal/provider"
)

// bridgeProcessは組み込みWORLDアダプタの常駐セッションを保持する。gateでprotocol v1の単一実行を保証し、ネイティブ状態を合成間で維持する。
type bridgeProcess struct {
	path     string
	provider string
	session  *provider.Session
}

var sharedBridge bridgeProcess
var bridgeGate = make(chan struct{}, 1)

func invokeBridge(ctx context.Context, bridge, jobPath, outputPath string) error {
	return InvokeReport(ctx, bridge, jobPath, outputPath, nil)
}

// InvokeReportはWORLDブリッジを実行し、任意でspeechタイミング報告を受け取る。
func InvokeReport(ctx context.Context, bridge, jobPath, outputPath string, report *[]provider.WorldSpeechResult) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case bridgeGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-bridgeGate }()
	if err := ctx.Err(); err != nil {
		return err
	}

	job, err := ReadBridgeJob(jobPath)
	if err != nil {
		return err
	}
	providerID, err := providerIDForEngine(job.Engine)
	if err != nil {
		return err
	}

	client := &sharedBridge
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

	if job.Speech && !slices.Contains(client.session.Hello().Capabilities, provider.CapabilityWorldSpeechV1) {
		return fmt.Errorf("WORLD bridge does not support speech timing; rebuild utautts-worldline-bridge")
	}
	if job.CodaRelease && !slices.Contains(client.session.Hello().Capabilities, provider.CapabilityCodaReleaseV1) {
		return fmt.Errorf("WORLD bridge does not support coda release timing; rebuild utautts-worldline-bridge")
	}
	result, err := client.session.Render(ctx, provider.RenderRequest{
		Contract:        "unit-renderer",
		ContractVersion: 1,
		InputPath:       jobPath,
		OutputPath:      outputPath,
	}, provider.RenderOptions{})
	if err != nil && !client.session.IsAlive() {
		client.stop()
	}
	if err == nil && report != nil {
		data, encodeErr := json.Marshal(result.Report["world_speech"])
		if encodeErr != nil {
			return encodeErr
		}
		if decodeErr := json.Unmarshal(data, report); decodeErr != nil {
			return fmt.Errorf("decode WORLD speech report: %w", decodeErr)
		}
		if job.Speech && *report == nil {
			return fmt.Errorf("WORLD bridge omitted speech report")
		}
	}
	return err
}

// BridgeJobはブリッジjobの検証済み要約。テストと診断で参照する。
type BridgeJob struct {
	CodaRelease bool
	Speech      bool
	Engine      string `json:"engine"`
}

// ReadBridgeJobはjobファイルのcontractを検証して要約を返す。
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

// Closeは常駐bridgeセッションを解放する。gateを取るため実行中renderの完了を待つ。
func Close() {
	bridgeGate <- struct{}{}
	sharedBridge.stop()
	<-bridgeGate
}

func (client *bridgeProcess) stop() {
	if client.session != nil {
		_ = client.session.Close()
	}
	client.path, client.provider, client.session = "", "", nil
}
