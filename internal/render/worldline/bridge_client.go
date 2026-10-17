//go:build !js

package worldline

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/provider"
)

// gateで同時実行を防ぎ、合成間でWORLDの常駐状態を維持する。
type bridgeProcess struct {
	path     string
	provider string
	session  *provider.Session
}

var sharedBridge bridgeProcess
var bridgeGate = make(chan struct{}, 1)

type scopedBridge struct {
	process bridgeProcess
	gate    chan struct{}
}

func InvokeReport(ctx context.Context, bridge, jobPath, outputPath string, report *[]provider.WorldSpeechResult) error {
	if ctx == nil {
		ctx = context.Background()
	}
	client, gate := &sharedBridge, bridgeGate
	owned, err := lifecycle.Resource(ctx, "worldline.bridge", (*scopedBridge)(nil),
		func() *scopedBridge { return &scopedBridge{gate: make(chan struct{}, 1)} },
		func(client *scopedBridge) error {
			client.gate <- struct{}{}
			defer func() { <-client.gate }()
			return client.process.stop()
		})
	if err != nil {
		return err
	}
	if owned != nil {
		client, gate = &owned.process, owned.gate
	}
	select {
	case gate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-gate }()
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
	if job.Anchors && !slices.Contains(client.session.Hello().Capabilities, provider.CapabilitySpeechAnchorsV1) {
		return fmt.Errorf("WORLD bridge does not support multilingual speech anchors; rebuild utautts-worldline-bridge")
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

func (client *bridgeProcess) stop() error {
	var err error
	if client.session != nil {
		err = client.session.Close()
	}
	client.path, client.provider, client.session = "", "", nil
	return err
}
