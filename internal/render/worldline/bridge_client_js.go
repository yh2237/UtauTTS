//go:build js

package worldline

import (
	"context"
	"fmt"
	"os"
	"sync"

	"utautts/internal/provider"
	"utautts/internal/worldrender"
)

var (
	rendererOnce sync.Once
	wasmRenderer *worldrender.Renderer
)

func sharedRenderer() *worldrender.Renderer {
	rendererOnce.Do(func() { wasmRenderer = worldrender.NewRenderer() })
	return wasmRenderer
}

// wasmは外部プロセスを起動せず、JSブリッジ経由でWORLDを呼ぶ。
func InvokeReport(ctx context.Context, bridge, jobPath, outputPath string, report *[]provider.WorldSpeechResult) error {
	_ = bridge
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := os.ReadFile(jobPath)
	if err != nil {
		return fmt.Errorf("read worldline job: %w", err)
	}
	results, err := sharedRenderer().RenderJob(data, outputPath)
	if err != nil {
		return fmt.Errorf("worldline wasm render: %w", err)
	}
	if report != nil {
		*report = results
	}
	return nil
}

func Close() {
	if wasmRenderer != nil {
		wasmRenderer.Close()
	}
}
