package base

import (
	"github.com/yh2237/UtauIDX/audio"
	"github.com/yh2237/UtauTTS/internal/plan"
)

type RenderFunc func(*plan.Plan, Config) (*audio.PCM, error)

// TestRendererを設定すると、WORLDの代わりにこれで描画する。rendertestが設定する。
var TestRenderer RenderFunc
