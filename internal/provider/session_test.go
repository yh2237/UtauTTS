package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	helperEnabled = "UTAUTTS_PROVIDER_HELPER"
	helperMode    = "UTAUTTS_PROVIDER_HELPER_MODE"
)

func TestProviderHelperProcess(t *testing.T) {
	if os.Getenv(helperEnabled) != "1" {
		return
	}
	encoder := json.NewEncoder(os.Stdout)
	mode := os.Getenv(helperMode)
	if mode == "bad-provider" {
		_ = encoder.Encode(Hello{Type: MessageHello, Protocol: ProtocolName, ProtocolVersion: ProtocolVersion, Provider: "wrong", ProviderVersion: "1", Session: true, Contracts: []ContractSupport{{Name: "unit-renderer", Version: 1}}})
		return
	}
	if mode == "bad-line" {
		_, _ = fmt.Fprintln(os.Stdout, "not json")
		return
	}
	_ = encoder.Encode(Hello{
		Type: MessageHello, Protocol: ProtocolName, ProtocolVersion: ProtocolVersion,
		Provider: "test-provider", ProviderVersion: "1", Session: true,
		Contracts: []ContractSupport{{Name: "unit-renderer", Version: 1}},
	})
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var header struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(scanner.Bytes(), &header) != nil {
			return
		}
		switch header.Type {
		case MessageRender:
			var request RenderRequest
			if json.Unmarshal(scanner.Bytes(), &request) != nil {
				return
			}
			if mode == "cancel-aware" {
				continue
			}
			_ = encoder.Encode(Progress{Type: MessageProgress, RequestID: request.RequestID, Phase: "render", Progress: 0.5})
			_ = encoder.Encode(Result{Type: MessageResult, RequestID: request.RequestID, Audio: AudioArtifact{Path: request.OutputPath, Format: "pcm_s16le", SampleRate: 44100, Channels: 1}})
		case MessageCancel:
			var cancel Cancel
			if json.Unmarshal(scanner.Bytes(), &cancel) != nil {
				return
			}
			_ = encoder.Encode(ErrorMessage{Type: MessageError, RequestID: cancel.RequestID, Code: "canceled", Message: "request canceled"})
		case MessageShutdown:
			return
		}
	}
}

func testSessionOptions(t *testing.T, mode string) SessionOptions {
	t.Helper()
	args := []string{"-test.run=^TestProviderHelperProcess$"}
	env := append(os.Environ(), helperEnabled+"=1", helperMode+"="+mode)
	return SessionOptions{
		Executable:      os.Args[0],
		Args:            args,
		Env:             env,
		Provider:        "test-provider",
		ProviderVersion: "1",
		Contract:        "unit-renderer",
		ContractVersion: 1,
		StartTimeout:    2 * time.Second,
		CancelGrace:     300 * time.Millisecond,
		ShutdownGrace:   time.Second,
	}
}

func TestSessionKeepsProviderResidentAcrossRequests(t *testing.T) {
	session, err := StartSession(context.Background(), testSessionOptions(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if !session.Hello().Session {
		t.Fatal("provider did not advertise session support")
	}
	for index := 0; index < 2; index++ {
		result, err := session.Render(context.Background(), RenderRequest{
			InputPath: "/tmp/input.json", OutputPath: "/tmp/output.wav",
		}, RenderOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if result.Audio.Format != "pcm_s16le" || result.Audio.SampleRate != 44100 {
			t.Fatalf("result = %#v", result)
		}
	}
}

func TestSessionReportsProgressAndDiagnostics(t *testing.T) {
	session, err := StartSession(context.Background(), testSessionOptions(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var progress Progress
	result, err := session.Render(context.Background(), RenderRequest{InputPath: "input", OutputPath: "output"}, RenderOptions{
		OnProgress: func(value Progress) { progress = value },
	})
	if err != nil {
		t.Fatal(err)
	}
	if progress.Phase != "render" || progress.Progress != 0.5 || result.RequestID == "" {
		t.Fatalf("progress=%#v result=%#v", progress, result)
	}
}

func TestSessionHandshakeRejectsProviderMismatch(t *testing.T) {
	_, err := StartSession(context.Background(), testSessionOptions(t, "bad-provider"))
	if err == nil || !strings.Contains(err.Error(), "provider id mismatch") {
		t.Fatalf("error = %v", err)
	}
}

func TestSessionHandshakeRejectsMissingCapability(t *testing.T) {
	options := testSessionOptions(t, "")
	options.Capabilities = []string{"frame_pitch"}
	_, err := StartSession(context.Background(), options)
	if err == nil || !strings.Contains(err.Error(), "required capability") {
		t.Fatalf("error = %v", err)
	}
}

func TestSessionCancellationKeepsResponsiveProviderAlive(t *testing.T) {
	session, err := StartSession(context.Background(), testSessionOptions(t, "cancel-aware"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = session.Render(ctx, RenderRequest{InputPath: "input", OutputPath: "output"}, RenderOptions{})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("close error = %v", err)
	}
}
