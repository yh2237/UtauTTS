package worldline

import (
	"context"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/provider"
	"github.com/yh2237/UtauTTS/internal/worldrender"
)

// WORLDは同じプロセスで動かし、読み込んだエンジンと分析結果を合成間で使い回す。そのため同時には1つだけ合成する。
var (
	worldRenderer = worldrender.NewRenderer()
	worldGate     = make(chan struct{}, 1)
)

func renderWorld(ctx context.Context, options provider.WorldlineOptions, enginePath string) (*audio.PCM, []provider.WorldSpeechResult, error) {
	select {
	case worldGate <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
	defer func() { <-worldGate }()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return worldRenderer.Render(ctx, options, enginePath)
}
