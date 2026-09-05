package voicebank

import (
	"path/filepath"
	"strings"
)

// StableID distinguishes identically named banks inside different archives.
func StableID(root, path string) string {
	if absolute, err := filepath.Abs(root); err == nil {
		root = absolute
	}
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.Base(path)
	}
	return filepath.ToSlash(rel)
}

// ResolveLegacyID accepts the old basename only if it is unambiguous.
func ResolveLegacyID[T any](items map[string]T, id string) (T, bool) {
	if value, ok := items[id]; ok {
		return value, true
	}
	var result T
	found := false
	for key, value := range items {
		if filepath.Base(filepath.FromSlash(key)) == id {
			if found {
				var zero T
				return zero, false
			}
			result, found = value, true
		}
	}
	return result, found
}
