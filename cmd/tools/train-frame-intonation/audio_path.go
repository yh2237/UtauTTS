package main

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveAudioPath(path, dataset, audioRoot string) string {
	normalized := strings.ReplaceAll(path, "\\", string(filepath.Separator))
	if filepath.IsAbs(normalized) {
		return normalized
	}
	candidates := []string{path, normalized}
	if audioRoot != "" {
		candidates = append(candidates, filepath.Join(audioRoot, normalized))
	}
	if dataset != "" {
		base, err := filepath.Abs(filepath.Dir(dataset))
		if err == nil {
			for {
				candidates = append(candidates, filepath.Join(base, normalized))
				parent := filepath.Dir(base)
				if parent == base {
					break
				}
				base = parent
			}
		}
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return path
}
