// Package providerは外部合成providerとのプロセス境界を提供する。契約固有の入力はプロトコルに埋め込まず、ホスト管理のjobファイルで渡す。
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

// ContractSupportはproviderが実装する1つの契約バージョンを示す。ハンドシェイクで複数契約を通知できる。
type ContractSupport struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
}

// Helloはproviderセッションが最初に送るメッセージ。
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

// RenderRequestは1つの描画ジョブを開始する。InputPath/OutputPathはホスト管理のjobディレクトリ内パスで、形式は選択した契約バージョンが定める。
type RenderRequest struct {
	Type            string `json:"type"`
	RequestID       string `json:"request_id"`
	Contract        string `json:"contract"`
	ContractVersion int    `json:"contract_version"`
	InputPath       string `json:"input_path"`
	OutputPath      string `json:"output_path"`
}

// Progressは実行中リクエストの進捗をbest-effortで報告する。
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

// AudioArtifactはproviderが書き出した結果を示す。Pathは通常絶対パスで、相対パスは同じjobディレクトリ基準で解釈される。
type AudioArtifact struct {
	Path       string `json:"path"`
	Format     string `json:"format"`
	SampleRate int    `json:"sample_rate"`
	Channels   int    `json:"channels"`
}

// Resultは1つの描画リクエストを完了させる。
type Result struct {
	Type      string         `json:"type"`
	RequestID string         `json:"request_id"`
	Audio     AudioArtifact  `json:"audio"`
	Report    map[string]any `json:"report,omitempty"`
}

// ErrorMessageは1リクエストまたはセッションハンドシェイクの終端エラー。
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

// Shutdownは書き出し済み出力をフラッシュ後、セッションの正常終了を求める。
type Shutdown struct {
	Type string `json:"type"`
}

type messageHeader struct {
	Type string `json:"type"`
}

// decodeMessageはプロトコル1行をデコードする。未知の型は拒否し、providerによる暗黙のダウングレードを防ぐ。
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
