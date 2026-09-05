package render

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"utautts/internal/processutil"
)

type worldlineBridgeResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type worldlineBridgeProcess struct {
	path   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

var sharedWorldlineBridge worldlineBridgeProcess
var worldlineBridgeGate = make(chan struct{}, 1)

func invokeWorldlineBridge(ctx context.Context, bridge, manifestPath string) error {
	client := &sharedWorldlineBridge
	select {
	case worldlineBridgeGate <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-worldlineBridgeGate }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if client.cmd == nil || client.path != bridge {
		client.stop()
		command := exec.Command(bridge, "--serve")
		processutil.Configure(command)
		stdin, err := command.StdinPipe()
		if err != nil {
			return err
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			return err
		}
		if err := command.Start(); err != nil {
			return err
		}
		client.path, client.cmd, client.stdin, client.stdout = bridge, command, stdin, bufio.NewReader(stdout)
	}
	if _, err := fmt.Fprintln(client.stdin, manifestPath); err != nil {
		client.stop()
		return err
	}
	responseLine := make(chan struct {
		line string
		err  error
	}, 1)
	stdout := client.stdout
	go func() {
		line, err := stdout.ReadString('\n')
		responseLine <- struct {
			line string
			err  error
		}{line, err}
	}()
	select {
	case <-ctx.Done():
		client.stop()
		return ctx.Err()
	case result := <-responseLine:
		if result.err != nil {
			client.stop()
			command := exec.CommandContext(ctx, bridge, manifestPath)
			processutil.Configure(command)
			output, err := command.CombinedOutput()
			if err != nil {
				return fmt.Errorf("%w: %s", err, output)
			}
			return nil
		}
		var response worldlineBridgeResponse
		if err := json.Unmarshal([]byte(result.line), &response); err != nil {
			client.stop()
			return fmt.Errorf("decode worldline bridge response: %w", err)
		}
		if !response.OK {
			return fmt.Errorf("%s", response.Error)
		}
		return nil
	}
}

func (client *worldlineBridgeProcess) stop() {
	if client.stdin != nil {
		_ = client.stdin.Close()
	}
	if client.cmd != nil && client.cmd.Process != nil {
		_ = client.cmd.Process.Kill()
		_ = client.cmd.Wait()
	}
	client.path, client.cmd, client.stdin, client.stdout = "", nil, nil, nil
}
