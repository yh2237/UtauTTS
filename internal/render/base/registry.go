package base

import (
	"github.com/yh2237/UtauTTS/internal/plan"
	"github.com/yh2237/audiodsp/wav"
)

type RenderFunc func(*plan.Plan, Config) (*wav.PCM, error)

// TestRendererを設定すると、WORLDの代わりにこれで描画する。rendertestが設定する。
var TestRenderer RenderFunc
