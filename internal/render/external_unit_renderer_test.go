package render

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/engine"
	"utautts/internal/plan"
	"utautts/internal/provider"
)

const externalRendererHelperEnabled = "UTAUTTS_EXTERNAL_RENDERER_HELPER"

func TestExternalRendererHelper(t *testing.T) {
	if os.Getenv(externalRendererHelperEnabled) != "1" {
		return
	}
	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(provider.Hello{
		Type: provider.MessageHello, Protocol: provider.ProtocolName, ProtocolVersion: provider.ProtocolVersion,
		Provider: "test.external", ProviderVersion: "1", Session: true,
		Capabilities: []string{provider.CapabilityUnitRendererJobV1},
		Contracts:    []provider.ContractSupport{{Name: "unit-renderer", Version: 1}},
	}); err != nil {
		return
	}
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var header struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(scanner.Bytes(), &header) != nil {
			return
		}
		switch header.Type {
		case provider.MessageRender:
			var request provider.RenderRequest
			if json.Unmarshal(scanner.Bytes(), &request) != nil {
				return
			}
			data, err := os.ReadFile(request.InputPath)
			if err != nil {
				return
			}
			var job externalUnitRendererJob
			if json.Unmarshal(data, &job) != nil || job.Version != unitRendererJobVersion || len(job.Plan) == 0 {
				return
			}
			if err := audio.WriteWav(request.OutputPath, &audio.PCM{SampleRate: 16000, Channels: 1, Data: []int16{1, 2, 3}}); err != nil {
				return
			}
			_ = encoder.Encode(provider.Diagnostic{Type: provider.MessageDiagnostic, RequestID: request.RequestID, Severity: "info", Code: "helper", Message: "rendered"})
			_ = encoder.Encode(provider.Result{
				Type: provider.MessageResult, RequestID: request.RequestID,
				Audio: provider.AudioArtifact{Path: request.OutputPath, Format: "wav_pcm_s16le", SampleRate: 16000, Channels: 1},
			})
		case provider.MessageShutdown:
			return
		}
	}
}

func TestExternalUnitRendererKeepsProviderResidentAcrossRenders(t *testing.T) {
	if err := os.Setenv(externalRendererHelperEnabled, "1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Unsetenv(externalRendererHelperEnabled) })
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	definition := engine.Definition{
		ID: "external-public", Contract: engine.ContractUnitRenderer, ContractVersion: 1,
		Provider: "test.external", ProviderVersion: "1", Protocol: provider.ProtocolName, ProtocolVersion: provider.ProtocolVersion,
		ProviderArgs: []string{"-test.run=^TestExternalRendererHelper$"},
		Resources:    map[engine.ResourceKey]string{engine.ResourceProviderExecutable: executable},
	}
	cleanupKey := externalProviderSessionKey(definition, executable)
	t.Cleanup(func() {
		externalProviderSessions.mu.Lock()
		session := externalProviderSessions.sessions[cleanupKey]
		delete(externalProviderSessions.sessions, cleanupKey)
		externalProviderSessions.mu.Unlock()
		if session != nil {
			_ = session.Close()
		}
	})

	input := &plan.Plan{Version: plan.Version, Voicebank: "bank", Units: []plan.Unit{{Source: "unit.wav", DurationMS: 100}}}
	for index := 0; index < 2; index++ {
		result, err := RenderWithReport(input, Config{
			Context: context.Background(), Backend: string(definition.Provider),
			Engine: engine.ResolvedEngine{
				Definition: definition,
				Provider:   engine.Provider{ID: definition.Provider, Contract: definition.Contract, Version: definition.ProviderVersion},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if result.Report.Provider != definition.Provider || len(result.Report.Diagnostics) != 1 || result.Audio.SampleRate != 16000 || len(result.Audio.Data) != 3 {
			t.Fatalf("render %d = %#v", index, result)
		}
	}
}
