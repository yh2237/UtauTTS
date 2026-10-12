package openjtalk

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
)

const helperProcessEnvironment = "UTAUTTS_OPENJTALK_BLOCKED_INPUT_HELPER"

func TestMain(m *testing.M) {
	switch os.Getenv(helperProcessEnvironment) {
	case "1":
		time.Sleep(3 * time.Second)
		os.Exit(0)
	case "echo":
		decoder, encoder := json.NewDecoder(os.Stdin), json.NewEncoder(os.Stdout)
		for {
			var request struct {
				Text string `json:"text"`
			}
			if err := decoder.Decode(&request); err == io.EOF {
				os.Exit(0)
			} else if err != nil {
				os.Exit(1)
			}
			if err := encoder.Encode(request); err != nil {
				os.Exit(1)
			}
		}
	}
	os.Exit(m.Run())
}

func TestHelperCancellationInterruptsBlockedWrite(t *testing.T) {
	t.Setenv(helperProcessEnvironment, "1")
	scope := lifecycle.New()
	t.Cleanup(func() { _ = scope.Close() })
	dictionary := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := invokeFrontendHelper(scope.Bind(ctx), os.Args[0], dictionary, strings.Repeat("a", 1<<20))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error=%v, want context.DeadlineExceeded", err)
	}
	if elapsed := time.Since(started); elapsed >= 2*time.Second {
		t.Errorf("blocked write took %s", elapsed)
	}
	t.Setenv(helperProcessEnvironment, "echo")
	data, err := invokeFrontendHelper(scope.Bind(context.Background()), os.Args[0], dictionary, "recover")
	if err != nil || string(data) != `{"text":"recover"}` {
		t.Fatalf("request after cancellation=%s err=%v", data, err)
	}
}
