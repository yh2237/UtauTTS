package voicebank

import (
	"path/filepath"
	"strings"
)

// StableIDは別アーカイブ内の同名音源を区別する。
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
