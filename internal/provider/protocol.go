// 入力はホスト管理のjobファイルで渡す。
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

// ハンドシェイクで対応する契約とバージョンを通知する。
type ContractSupport struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

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

// 入出力パスはjobディレクトリ内。形式は契約バージョンに従う。
type RenderRequest struct {
	Type            string `json:"type"`
	RequestID       string `json:"request_id"`
	Contract        string `json:"contract"`
	ContractVersion int    `json:"contract_version"`
	InputPath       string `json:"input_path"`
	OutputPath      string `json:"output_path"`
}

type Progress struct {
	Type      string  `json:"type"`
	RequestID string  `json:"request_id"`
	Phase     string  `json:"phase,omitempty"`
	Progress  float64 `json:"progress,omitempty"`
	Message   string  `json:"message,omitempty"`
}

// Diagnosticはホストのレポート/ログ向けの構造化ログで、終端応答ではない。
type Diagnostic struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Severity  string `json:"severity"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message"`
}

// 結果の相対パスはjobディレクトリ基準。
type AudioArtifact struct {
	Path       string `json:"path"`
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
}

type Result struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Audio     AudioArtifact  `json:"audio"`
	Report    map[string]any `json:"report,omitempty"`
}

type ErrorMessage struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable,omitempty"`
}

// Cancelは実行中リクエストの停止を求める。providerはcode "canceled"を返しセッションを維持してよい。
type Cancel struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id"`
}

type Shutdown struct {
	Type string `json:"type"`
}

type messageHeader struct {
	Type string `json:"type"`
}

// 未知の型を拒否し、暗黙のプロトコル変更を防ぐ。
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
