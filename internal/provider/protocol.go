// Package provider contains the process boundary used by external synthesis
// providers. The wire format is deliberately small; contract-specific input
// is passed as a host-owned job file rather than embedded as large JSON or
// binary data in the protocol stream.
package provider

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
)

const (
	ProtocolName    = "utautts-provider"
	ProtocolVersion = 1

	MessageHello      = "hello"
	MessageRender     = "render"
	MessageProgress   = "progress"
	MessageDiagnostic = "diagnostic"
	MessageResult     = "result"
	MessageError      = "error"
	MessageCancel     = "cancel"
	MessageShutdown   = "shutdown"
)

// ContractSupport declares one contract version implemented by a provider.
// A provider may advertise more than one contract in its handshake.
type ContractSupport struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// Hello is the first message emitted by a provider session.
type Hello struct {
	Type            string            `json:"type"`
	Protocol        string            `json:"protocol"`
	ProtocolVersion int               `json:"protocol_version"`
	Provider        string            `json:"provider"`
	ProviderVersion string            `json:"provider_version"`
	Session         bool              `json:"session"`
	Capabilities    []string          `json:"capabilities,omitempty"`
	Contracts       []ContractSupport `json:"contracts"`
}

// RenderRequest starts one render job. InputPath and OutputPath are paths in
// a host-owned job directory. Their file format is defined by the selected
// contract version, not by the transport itself.
type RenderRequest struct {
	Type            string `json:"type"`
	RequestID       string `json:"request_id"`
	Contract        string `json:"contract"`
	ContractVersion int    `json:"contract_version"`
	InputPath       string `json:"input_path"`
	OutputPath      string `json:"output_path"`
}

// Progress reports best-effort progress for the active request.
type Progress struct {
	Type      string  `json:"type"`
	RequestID string  `json:"request_id"`
	Phase     string  `json:"phase,omitempty"`
	Progress  float64 `json:"progress,omitempty"`
	Message   string  `json:"message,omitempty"`
}

// Diagnostic is a structured provider log intended for the host's report or
// log view. It is not a terminal response.
type Diagnostic struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Severity  string `json:"severity"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
}

// AudioArtifact describes the result written by a provider. Path is normally
// absolute because the host sends an absolute job path; a relative path is
// interpreted relative to that same host-owned job directory by the contract
// adapter.
type AudioArtifact struct {
	Path       string `json:"path"`
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
}

// Result completes one render request.
type Result struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Audio     AudioArtifact  `json:"audio"`
	Report    map[string]any `json:"report,omitempty"`
}

// ErrorMessage is a terminal provider error for one request or for the
// session handshake.
type ErrorMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

// Cancel asks the provider to stop one active request. A provider may return
// an error with code "canceled" and keep the session alive.
type Cancel struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
}

// Shutdown asks a session to exit cleanly after all already-written output
// has been flushed.
type Shutdown struct {
	Type string `json:"type"`
}

type messageHeader struct {
	Type string `json:"type"`
}

// decodeMessage decodes exactly one protocol line. Unknown message types are
// rejected so a provider cannot silently downgrade the host's expectations.
func decodeMessage(data []byte) (any, error) {
	var header messageHeader
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("decode provider message: %w", err)
	}
	if header.Type == "" {
		return nil, fmt.Errorf("provider message has no type")
	}
	var message any
	switch header.Type {
	case MessageHello:
		message = new(Hello)
	case MessageProgress:
		message = new(Progress)
	case MessageDiagnostic:
		message = new(Diagnostic)
	case MessageResult:
		message = new(Result)
	case MessageError:
		message = new(ErrorMessage)
	default:
		return nil, fmt.Errorf("unknown provider message type %q", header.Type)
	}
	if err := json.Unmarshal(data, message); err != nil {
		return nil, fmt.Errorf("decode provider %s message: %w", header.Type, err)
	}
	return message, nil
}

func writeMessage(writer io.Writer, message any) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}
	if _, err := writer.Write(append(data, '\n')); err != nil {
		return err
	}
	return nil
}

func scanMessages(reader io.Reader, maxLineBytes int, messages chan<- []byte, stopped <-chan struct{}, readErr chan<- error) {
	scanner := bufio.NewScanner(reader)
	bufferSize := 64 * 1024
	if maxLineBytes < bufferSize {
		bufferSize = maxLineBytes
	}
	scanner.Buffer(make([]byte, bufferSize), maxLineBytes)
	defer close(messages)
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		select {
		case messages <- line:
		case <-stopped:
			return
		}
	}
	if err := scanner.Err(); err != nil {
		select {
		case readErr <- err:
		case <-stopped:
		}
	}
}
