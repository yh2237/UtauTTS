package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"utautts/internal/processutil"
)

const (
	defaultStartTimeout  = 5 * time.Second
	defaultCancelGrace   = 750 * time.Millisecond
	defaultShutdownGrace = 2 * time.Second
	defaultMaxLineBytes  = 16 * 1024 * 1024
)

// SessionOptions describes the executable and the handshake expected by the
// host. Args are passed directly to exec.Command; no shell expansion occurs.
type SessionOptions struct {
	Executable      string
	Args            []string
	Dir             string
	Env             []string
	Provider        string
	ProviderVersion string
	Capabilities    []string
	Contract        string
	ContractVersion int
	ProtocolVersion int
	StartTimeout    time.Duration
	CancelGrace     time.Duration
	ShutdownGrace   time.Duration
	MaxLineBytes    int
}

func (options *SessionOptions) normalize() error {
	options.Executable = strings.TrimSpace(options.Executable)
	options.Provider = strings.TrimSpace(options.Provider)
	options.ProviderVersion = strings.TrimSpace(options.ProviderVersion)
	options.Contract = strings.TrimSpace(options.Contract)
	capabilities := make([]string, 0, len(options.Capabilities))
	seenCapabilities := make(map[string]struct{}, len(options.Capabilities))
	for _, capability := range options.Capabilities {
		capability = strings.TrimSpace(capability)
		if capability == "" {
			return errors.New("provider capability must not be empty")
		}
		if _, exists := seenCapabilities[capability]; !exists {
			seenCapabilities[capability] = struct{}{}
			capabilities = append(capabilities, capability)
		}
	}
	options.Capabilities = capabilities
	if options.Executable == "" {
		return errors.New("provider executable is required")
	}
	if options.Provider == "" {
		return errors.New("provider id is required")
	}
	if options.Contract == "" {
		return errors.New("provider contract is required")
	}
	if options.ContractVersion <= 0 {
		return errors.New("provider contract version must be positive")
	}
	if options.ProtocolVersion == 0 {
		options.ProtocolVersion = ProtocolVersion
	}
	if options.ProtocolVersion != ProtocolVersion {
		return fmt.Errorf("unsupported provider protocol version %d", options.ProtocolVersion)
	}
	if options.StartTimeout <= 0 {
		options.StartTimeout = defaultStartTimeout
	}
	if options.CancelGrace <= 0 {
		options.CancelGrace = defaultCancelGrace
	}
	if options.ShutdownGrace <= 0 {
		options.ShutdownGrace = defaultShutdownGrace
	}
	if options.MaxLineBytes <= 0 {
		options.MaxLineBytes = defaultMaxLineBytes
	}
	if options.MaxLineBytes < 1024 {
		return errors.New("provider max line size must be at least 1024 bytes")
	}
	return nil
}

// RenderOptions receives best-effort non-terminal messages from the provider.
type RenderOptions struct {
	OnProgress   func(Progress)
	OnDiagnostic func(Diagnostic)
}

// RemoteError is an error reported by the provider process.
type RemoteError struct {
	Code      string
	Message   string
	Retryable bool
}

func (err *RemoteError) Error() string {
	if err.Code == "" {
		return "provider error: " + err.Message
	}
	return fmt.Sprintf("provider error %s: %s", err.Code, err.Message)
}

// Session is a single long-lived provider process. Only one render request is
// in flight at a time in protocol v1, but the same process may handle many
// sequential requests and keep its model/runtime state resident.
type Session struct {
	options SessionOptions
	hello   Hello

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	lines   chan []byte
	readErr chan error
	stopCh  chan struct{}
	exitCh  chan struct{}

	stderr boundedBuffer

	writeMu  sync.Mutex
	renderMu sync.Mutex
	stopOnce sync.Once
	exitMu   sync.Mutex
	exitErr  error
}

var requestSequence atomic.Uint64

// StartSession starts a provider and waits for its hello message.
func StartSession(ctx context.Context, options SessionOptions) (*Session, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := options.normalize(); err != nil {
		return nil, err
	}
	command := exec.Command(options.Executable, options.Args...)
	command.Dir = options.Dir
	if options.Env != nil {
		command.Env = append([]string(nil), options.Env...)
	}
	processutil.Configure(command)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open provider stdin: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open provider stdout: %w", err)
	}
	session := &Session{
		options: options,
		cmd:     command,
		stdin:   stdin,
		lines:   make(chan []byte),
		readErr: make(chan error, 1),
		stopCh:  make(chan struct{}),
		exitCh:  make(chan struct{}),
		stderr:  boundedBuffer{limit: 64 * 1024},
	}
	command.Stderr = &session.stderr
	if err := command.Start(); err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("start provider %q: %w", options.Executable, err)
	}
	go session.waitProcess()
	go scanMessages(stdout, options.MaxLineBytes, session.lines, session.stopCh, session.readErr)

	startCtx, cancel := context.WithTimeout(ctx, options.StartTimeout)
	defer cancel()
	message, err := session.next(startCtx)
	if err != nil {
		session.stop()
		return nil, fmt.Errorf("provider handshake: %w", err)
	}
	hello, ok := message.(*Hello)
	if !ok {
		session.stop()
		return nil, fmt.Errorf("provider handshake expected %q, got %T", MessageHello, message)
	}
	if err := validateHello(*hello, options); err != nil {
		session.stop()
		return nil, err
	}
	session.hello = *hello
	return session, nil
}

func validateHello(hello Hello, options SessionOptions) error {
	if hello.Type != MessageHello {
		return fmt.Errorf("provider handshake has type %q", hello.Type)
	}
	if hello.Protocol != ProtocolName || hello.ProtocolVersion != options.ProtocolVersion {
		return fmt.Errorf("provider protocol mismatch: got %s/%d, want %s/%d", hello.Protocol, hello.ProtocolVersion, ProtocolName, options.ProtocolVersion)
	}
	if hello.Provider != options.Provider {
		return fmt.Errorf("provider id mismatch: got %q, want %q", hello.Provider, options.Provider)
	}
	if options.ProviderVersion != "" && hello.ProviderVersion != options.ProviderVersion {
		return fmt.Errorf("provider version mismatch: got %q, want %q", hello.ProviderVersion, options.ProviderVersion)
	}
	advertisedCapabilities := make(map[string]struct{}, len(hello.Capabilities))
	for _, capability := range hello.Capabilities {
		advertisedCapabilities[strings.TrimSpace(capability)] = struct{}{}
	}
	for _, capability := range options.Capabilities {
		if _, exists := advertisedCapabilities[capability]; !exists {
			return fmt.Errorf("provider does not advertise required capability %q", capability)
		}
	}
	if !hello.Session {
		return errors.New("provider does not support session mode")
	}
	for _, contract := range hello.Contracts {
		if contract.Name == options.Contract && contract.Version == options.ContractVersion {
			return nil
		}
	}
	return fmt.Errorf("provider does not support contract %q version %d", options.Contract, options.ContractVersion)
}

// Hello returns the validated provider handshake.
func (session *Session) Hello() Hello {
	return session.hello
}

// IsAlive reports whether the provider process is still running. A session
// that has exited cannot be reused; the caller should create a new session.
func (session *Session) IsAlive() bool {
	return session != nil && !session.isExited()
}

// Render sends one request and waits for its result. The session remains alive
// after a successful result and can be reused for the next request.
func (session *Session) Render(ctx context.Context, request RenderRequest, options RenderOptions) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	session.renderMu.Lock()
	defer session.renderMu.Unlock()
	if request.Type == "" {
		request.Type = MessageRender
	}
	if request.Type != MessageRender {
		return Result{}, fmt.Errorf("invalid provider request type %q", request.Type)
	}
	if request.RequestID == "" {
		request.RequestID = fmt.Sprintf("request-%d", requestSequence.Add(1))
	}
	if request.Contract == "" {
		request.Contract = session.options.Contract
	}
	if request.ContractVersion == 0 {
		request.ContractVersion = session.options.ContractVersion
	}
	if strings.TrimSpace(request.InputPath) == "" || strings.TrimSpace(request.OutputPath) == "" {
		return Result{}, errors.New("provider render request requires input_path and output_path")
	}
	if err := session.send(request); err != nil {
		return Result{}, err
	}
	for {
		message, err := session.next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				session.cancelRequest(request.RequestID)
				return Result{}, ctx.Err()
			}
			return Result{}, err
		}
		switch message := message.(type) {
		case *Progress:
			if message.RequestID == request.RequestID && options.OnProgress != nil {
				options.OnProgress(*message)
			}
		case *Diagnostic:
			if message.RequestID == "" || message.RequestID == request.RequestID {
				if options.OnDiagnostic != nil {
					options.OnDiagnostic(*message)
				}
			}
		case *Result:
			if message.RequestID != request.RequestID {
				return Result{}, fmt.Errorf("provider result request id %q does not match %q", message.RequestID, request.RequestID)
			}
			return *message, nil
		case *ErrorMessage:
			if message.RequestID != "" && message.RequestID != request.RequestID {
				return Result{}, fmt.Errorf("provider error request id %q does not match %q", message.RequestID, request.RequestID)
			}
			return Result{}, &RemoteError{Code: message.Code, Message: message.Message, Retryable: message.Retryable}
		case *Hello:
			return Result{}, errors.New("provider sent a second hello during render")
		default:
			return Result{}, fmt.Errorf("unsupported provider message %T", message)
		}
	}
}

func (session *Session) cancelRequest(requestID string) {
	_ = session.send(Cancel{Type: MessageCancel, RequestID: requestID})
	timer := time.NewTimer(session.options.CancelGrace)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			session.stop()
			return
		case line, ok := <-session.lines:
			if !ok {
				session.stop()
				return
			}
			message, err := decodeMessage(line)
			if err != nil {
				session.stop()
				return
			}
			switch message := message.(type) {
			case *Diagnostic, *Progress:
				continue
			case *Result:
				if message.RequestID == requestID {
					return
				}
			case *ErrorMessage:
				if message.RequestID == "" || message.RequestID == requestID {
					return
				}
			default:
				session.stop()
				return
			}
		}
	}
}

// Close asks the provider to exit and kills it if it does not comply within
// the shutdown grace period. It is safe to call more than once.
func (session *Session) Close() error {
	session.renderMu.Lock()
	defer session.renderMu.Unlock()
	if session.isExited() {
		return session.exitError()
	}
	_ = session.send(Shutdown{Type: MessageShutdown})
	timer := time.NewTimer(session.options.ShutdownGrace)
	defer timer.Stop()
	select {
	case <-session.exitCh:
		return session.exitError()
	case <-timer.C:
		session.stop()
		return session.exitError()
	}
}

func (session *Session) send(message any) error {
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	if session.stdin == nil {
		return errors.New("provider session is closed")
	}
	if err := writeMessage(session.stdin, message); err != nil {
		session.stop()
		return fmt.Errorf("write provider message: %w", err)
	}
	return nil
}

func (session *Session) next(ctx context.Context) (any, error) {
	select {
	case line, ok := <-session.lines:
		if !ok {
			if err := session.readError(); err != nil {
				return nil, fmt.Errorf("read provider output: %w", err)
			}
			return nil, fmt.Errorf("provider exited: %s", session.stderr.String())
		}
		return decodeMessage(line)
	case <-session.exitCh:
		if err := session.exitError(); err != nil {
			return nil, fmt.Errorf("provider exited: %w: %s", err, session.stderr.String())
		}
		return nil, fmt.Errorf("provider exited: %s", session.stderr.String())
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (session *Session) waitProcess() {
	err := session.cmd.Wait()
	session.exitMu.Lock()
	session.exitErr = err
	session.exitMu.Unlock()
	close(session.exitCh)
}

func (session *Session) isExited() bool {
	select {
	case <-session.exitCh:
		return true
	default:
		return false
	}
}

func (session *Session) exitError() error {
	session.exitMu.Lock()
	defer session.exitMu.Unlock()
	return session.exitErr
}

func (session *Session) stop() {
	session.stopOnce.Do(func() {
		close(session.stopCh)
		if session.stdin != nil {
			_ = session.stdin.Close()
		}
		if session.cmd != nil && session.cmd.Process != nil && !session.isExited() {
			_ = session.cmd.Process.Kill()
		}
	})
}

type boundedBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

func (buffer *boundedBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	length := len(data)
	remaining := buffer.limit - len(buffer.data)
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		buffer.data = append(buffer.data, data...)
	}
	return length, nil
}

func (buffer *boundedBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return string(buffer.data)
}

func (session *Session) readError() error {
	select {
	case err := <-session.readErr:
		return err
	default:
		return nil
	}
}
