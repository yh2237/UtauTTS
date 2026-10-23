package openjtalk

import (
	"context"
	"errors"
	"testing"
)

func TestAnalyzeContextHonorsCancellationBeforeResolvingRuntime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := AnalyzeContext(ctx, "テスト", Config{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestAnalyzeRejectsEmptyTextBeforeResolvingRuntime(t *testing.T) {
	if _, err := Analyze("  ", Config{}); err == nil {
		t.Fatal("empty text was accepted")
	}
}

func TestResolveEngineRejectsMissingExplicitPath(t *testing.T) {
	if _, err := ResolveEnginePath(t.TempDir()); err == nil {
		t.Fatal("directory was accepted as engine library")
	}
}
