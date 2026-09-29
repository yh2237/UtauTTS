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

// InvokeReportはwasmではブリッジプロセスを起動せず、同一プロセス内でWORLDレンダリングする。
// WORLD自体はJSブリッジ（globalThis.utauttsWorld）経由で呼ばれる。
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

// Closeはwasmレンダラを解放する。
func Close() {
	if wasmRenderer != nil {
		wasmRenderer.Close()
	}
}
