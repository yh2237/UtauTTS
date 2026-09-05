package plugin

import (
	"os"
	"path/filepath"
	"runtime"
)

// BuiltinRenderersはバックエンド機能とランタイム名の定義元。
func BuiltinRenderers() []Renderer {
	items := []Renderer{
		{ID: "utautts-world-phrase", DisplayName: "UtauTTS WORLD phrase", Backend: "utautts-world-phrase", DefaultPriority: 300, Assets: map[string]string{"world_engine": runtimeAsset("world-engine"), "worldline_bridge": runtimeAsset("worldline-bridge")}, Capabilities: Capabilities{FramePitch: true}},
		{ID: "openutau-worldline-r-faithful", DisplayName: "OpenUTAU WORLDLINE-R faithful", Backend: "openutau-worldline-r-faithful", DefaultPriority: 100, Assets: map[string]string{"worldline": runtimeAsset("worldline"), "worldline_bridge": runtimeAsset("worldline-bridge")}, Capabilities: Capabilities{FramePitch: true}},
		{ID: "waveform", DisplayName: "Waveform", Backend: "waveform", DefaultPriority: 0, Capabilities: Capabilities{FramePitch: true, BoundaryBridge: true}},
		{ID: "classic-utau", DisplayName: "Classic UTAU", Backend: "utau-external-resampler", DefaultPriority: -100, Capabilities: Capabilities{FramePitch: true}},
		{ID: "diffsinger", DisplayName: "DiffSinger (experimental)", Backend: "diffsinger", DefaultPriority: -50, Experimental: true, Assets: map[string]string{"diffsinger_bridge": runtimeAsset("diffsinger-bridge")}, Capabilities: Capabilities{FramePitch: true}},
	}
	if runtime.GOOS == "windows" {
		items = append(items, Renderer{ID: "utautts-world-phrase-cuda", DisplayName: "UtauTTS WORLD phrase (CUDA, experimental)", Backend: "utautts-world-phrase-cuda", DefaultPriority: -200, Experimental: true, Acceleration: "cuda", Assets: map[string]string{"world_engine": runtimeAsset("world-engine"), "worldline_bridge": runtimeAsset("worldline-bridge"), "world_gpu": runtimeAsset("waveform-gpu")}, Capabilities: Capabilities{FramePitch: true}})
	}
	for i := range items {
		switch items[i].ID {
		case "openutau-worldline-r-faithful":
			items[i].DefaultPriority = 200
		case "waveform":
			items[i].DefaultPriority = 100
		}
		items[i].ManifestVersion, items[i].Kind, items[i].Version, items[i].BuiltIn = 2, "renderer", "1", true
		if items[i].Acceleration == "" {
			items[i].Acceleration = "cpu"
		}
	}
	if runtime.GOOS != "windows" {
		filtered := items[:0]
		for _, item := range items {
			if item.ID != "diffsinger" {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	return items
}

func runtimeAsset(id string) string {
	names := map[string]string{"world-engine": "utautts-world-engine.so", "worldline-bridge": "utautts-worldline-bridge", "worldline": "libworldline.so", "diffsinger-bridge": "utautts-diffsinger-bridge", "waveform-gpu": "utautts-waveform-gpu.dll"}
	if runtime.GOOS == "windows" {
		names["world-engine"], names["worldline"], names["worldline-bridge"], names["diffsinger-bridge"] = "utautts-world-engine.dll", "worldline.dll", "utautts-worldline-bridge.exe", "utautts-diffsinger-bridge.exe"
	}
	name := names[id]
	if name == "" {
		return ""
	}
	var roots []string
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		if filepath.Base(root) == "app" || filepath.Base(root) == "tools" {
			root = filepath.Dir(root)
		}
		roots = append(roots, root)
	}
	if current, err := os.Getwd(); err == nil {
		roots = append(roots, workspaceRoot(current))
	}
	for _, root := range roots {
		path := filepath.Join(root, "runtime", name)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path
		}
	}
	if len(roots) > 0 {
		return filepath.Join(roots[0], "runtime", name)
	}
	return filepath.Join("runtime", name)
}
