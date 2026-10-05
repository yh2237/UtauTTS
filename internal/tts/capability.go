package tts

import (
	"math"

	"utautts/internal/plugin"
	"utautts/internal/prosody"
	"utautts/internal/render"
)

func rendererSupportsFramePitch(renderer string, capabilities *plugin.Capabilities) bool {
	return rendererCapability(renderer, capabilities, func(c plugin.Capabilities) bool { return c.FramePitch })
}

func rendererInternalTiming(renderer string, capabilities *plugin.Capabilities) bool {
	return rendererCapability(renderer, capabilities, func(c plugin.Capabilities) bool { return c.InternalTiming })
}

// rendererCapabilityは解決済みcapabilityを優先し、未解決時は外部manifestだけを参照する。Go側の既定値は持たない。
func rendererCapability(renderer string, capabilities *plugin.Capabilities, selectCapability func(plugin.Capabilities) bool) bool {
	if capabilities != nil {
		return selectCapability(*capabilities)
	}
	directories, _ := plugin.DefaultDirectories()
	items, _ := plugin.DiscoverRenderers(directories, nil)
	for _, item := range items {
		if item.ID == renderer || item.Provider == renderer {
			return selectCapability(item.Capabilities)
		}
	}
	return false
}

func applyPitchEnabled(cfg Config) bool {
	return cfg.ApplyPitch || cfg.ProsodyPitchOnly
}

func shouldPredictFrameContour(cfg Config, model *prosody.Model) bool {
	return applyPitchEnabled(cfg) && model != nil && model.HasFrameContour() &&
		rendererSupportsFramePitch(cfg.Renderer, cfg.RendererCapabilities)
}

func effectiveIntonationStrength(cfg Config) float64 {
	if !applyPitchEnabled(cfg) {
		return 0
	}
	// 自動輪郭が無いときは、強さが音源ピッチ安定化の指数になるため、レンダラーの上限で止める。
	return math.Min(cfg.IntonationStrength, render.MaxIntonationStrength)
}

// 自動曲線使用時も音源由来の補正を弱く残す。
const automaticSourceIntonationBlend = 0.25

func rendererIntonationStrength(cfg Config, automatic *render.PitchCurve) float64 {
	if automatic != nil {
		if !applyPitchEnabled(cfg) {
			return 0
		}
		return automaticSourceIntonationBlend
	}
	return effectiveIntonationStrength(cfg)
}
