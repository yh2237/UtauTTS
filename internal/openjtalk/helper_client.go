package openjtalk

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/processutil"
)

type frontendHelperProcess struct {
	mutex  sync.Mutex
	key    string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
}

var sharedFrontendHelper frontendHelperProcess

func invokeFrontendHelper(ctx context.Context, helper, dictionary, text string) ([]byte, error) {
	client, err := lifecycle.Resource(ctx, "openjtalk.helper", &sharedFrontendHelper,
		func() *frontendHelperProcess { return &frontendHelperProcess{} },
		func(client *frontendHelperProcess) error {
			client.mutex.Lock()
			defer client.mutex.Unlock()
			client.stop()
			return nil
		})
	if err != nil {
		return nil, err
	}
	client.mutex.Lock()
	defer client.mutex.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	key := helper + "\x00" + dictionary
	if client.cmd == nil || client.key != key {
		client.stop()
		command := exec.Command(helper, "--dictionary", dictionary, "--serve")
		processutil.Configure(command)
		stdin, err := command.StdinPipe()
		if err != nil {
			return nil, err
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			_ = stdin.Close()
			return nil, err
		}
		if err := command.Start(); err != nil {
			_ = stdin.Close()
			_ = stdout.Close()
			return nil, err
		}
		client.key, client.cmd, client.stdin, client.stdout = key, command, stdin, bufio.NewReader(stdout)
	}
	request, _ := json.Marshal(struct {
		Text string `json:"text"`
	}{text})
	command := client.cmd
	writeStopped := make(chan struct{})
	stopWriting := context.AfterFunc(ctx, func() {
		_ = command.Process.Kill()
		close(writeStopped)
	})
	_, writeErr := client.stdin.Write(append(request, '\n'))
	if !stopWriting() {
		<-writeStopped
	}
	if err := ctx.Err(); err != nil {
		client.stop()
		return nil, err
	}
	if writeErr != nil {
		client.stop()
		return nil, writeErr
	}
	type readResult struct {
		data []byte
		err  error
	}
	resultChannel := make(chan readResult, 1)
	stdout := client.stdout
	go func() {
		line, err := stdout.ReadBytes('\n')
		resultChannel <- readResult{line, err}
	}()
	select {
	case <-ctx.Done():
		client.stop()
		return nil, ctx.Err()
	case result := <-resultChannel:
		if result.err != nil {
			client.stop()
			return invokeFrontendHelperOnce(ctx, helper, dictionary, request)
		}
		var failure struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(result.data, &failure) == nil && failure.Error != "" {
			return nil, fmt.Errorf("%s", failure.Error)
		}
		return bytes.TrimSpace(result.data), nil
	}
}

func invokeFrontendHelperOnce(ctx context.Context, helper, dictionary string, request []byte) ([]byte, error) {
	command := exec.CommandContext(ctx, helper, "--dictionary", dictionary)
	processutil.Configure(command)
	command.Stdin = bytes.NewReader(request)
	return command.Output()
}

func (client *frontendHelperProcess) stop() {
	if client.stdin != nil {
		_ = client.stdin.Close()
	}
	if client.cmd != nil && client.cmd.Process != nil {
		_ = client.cmd.Process.Kill()
		_ = client.cmd.Wait()
	}
	client.key, client.cmd, client.stdin, client.stdout = "", nil, nil, nil
}
