package lifecycle

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestCloseCancelsAndWaitsForRequest(t *testing.T) {
	scope := New()
	ctx, release, err := scope.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	closed := make(chan struct{})
	closeErr := errors.New("close failed")
	if _, err := Resource(ctx, "owned", new(0), func() *int { return new(1) }, func(*int) error {
		close(closed)
		return closeErr
	}); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	go func() { results <- scope.Close() }()
	go func() { results <- scope.Close() }()
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not cancel active request")
	}
	select {
	case <-closed:
		t.Fatal("resource closed before request finished")
	default:
	}
	release()
	for range 2 {
		select {
		case err := <-results:
			if !errors.Is(err, closeErr) {
				t.Errorf("Close error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Close did not finish")
		}
	}
	if _, _, err := scope.Begin(context.Background()); !errors.Is(err, ErrClosed) {
		t.Fatalf("Begin after Close: %v", err)
	}
}

func TestResetAndNestedRequestOwnership(t *testing.T) {
	scope, other := New(), New()
	defer scope.Close()
	defer other.Close()
	ctx, release, err := scope.Begin(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	resourceFor := func(ctx context.Context) *int {
		value, err := Resource(ctx, "cache", (*int)(nil), func() *int { return new(1) }, nil)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	value, otherValue := resourceFor(ctx), resourceFor(other.Bind(nil))
	if value == otherValue {
		t.Fatal("scopes share a mutable resource")
	}
	resetting := make(chan struct{})
	reloaded := make(chan error, 1)
	go func() {
		close(resetting)
		reloaded <- scope.Reset(nil)
	}()
	<-resetting
	// Reset待機中の入れ子要求で読込ロックを取り直すとデッドロックする。
	nested := make(chan error, 1)
	go func() {
		nestedCtx, done, err := scope.Begin(ctx)
		if err == nil {
			done()
			if nestedCtx != ctx {
				err = errors.New("nested request lost context")
			}
		}
		nested <- err
	}()
	select {
	case err := <-nested:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("nested operation blocked by Reset")
	}
	release()
	if err := <-reloaded; err != nil {
		t.Fatal(err)
	}
	if resourceFor(scope.Bind(nil)) == value || resourceFor(other.Bind(nil)) != otherValue {
		t.Fatal("Reset did not replace only its own resources")
	}
}
