package render

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCanceledBridgeWaitReturnsBeforeActiveRender(t *testing.T) {
	worldlineBridgeGate <- struct{}{}
	defer func() { <-worldlineBridgeGate }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := make(chan error, 1)
	go func() { done <- invokeWorldlineBridge(ctx, "must-not-start", "") }()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled request blocked behind active render")
	}
}
