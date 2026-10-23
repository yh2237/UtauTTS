// Package pluginは、同梱と追加の抑揚モデルを探す。
package plugin

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yh2237/UtauTTS/internal/prosody"
)

type Model struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	Description      string `json:"description,omitempty"`
	Language         string `json:"language,omitempty"`
	Path             string `json:"path"`
	Version          int    `json:"version"`
	FeatureVersion   int    `json:"feature_version"`
	Mode             string `json:"mode"`
	SHA256           string `json:"sha256"`
	DefaultPriority  int    `json:"default_priority,omitempty"`
	RequiresFeatures bool   `json:"requires_features,omitempty"`
	FrameContour     bool   `json:"frame_contour,omitempty"`
}

type Catalog struct {
	Problems []string `json:"problems,omitempty"`
	Models   []Model
}

// DiscoverWithDefaultsは明示ディレクトリを優先し、同梱の既定ディレクトリも探索する。
func DiscoverWithDefaults(modelDirectories []string) *Catalog {
	models, err := DiscoverModels(append(modelDirectories, DefaultModelDirectories()...))
	catalog := &Catalog{Models: models}
	if err != nil {
		catalog.Problems = append(catalog.Problems, err.Error())
	}
	return catalog
}

func DiscoverModels(directories []string) ([]Model, error) {
	seenIDs, seenPaths := map[string]string{}, map[string]bool{}
	var result []Model
	var problems []error
	for _, directory := range uniqueDirectories(directories) {
		entries, err := os.ReadDir(directory)
		if err != nil {
			if !os.IsNotExist(err) {
				problems = append(problems, fmt.Errorf("read model directory %q: %w", directory, err))
			}
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
				continue
			}
			path, err := filepath.Abs(filepath.Join(directory, entry.Name()))
			if err != nil || seenPaths[strings.ToLower(path)] {
				continue
			}
			data, err := os.ReadFile(path)
			if err != nil {
				problems = append(problems, fmt.Errorf("read model %q: %w", path, err))
				continue
			}
			var identity struct {
				ID          string `json:"id"`
				DisplayName string `json:"display_name"`
			}
			if err := json.Unmarshal(data, &identity); err != nil {
				problems = append(problems, fmt.Errorf("decode model %q: %w", path, err))
				continue
			}
			if strings.TrimSpace(identity.ID) == "" || strings.TrimSpace(identity.DisplayName) == "" {
				continue
			}
			loaded, err := prosody.LoadModel(path)
			if err != nil {
				problems = append(problems, fmt.Errorf("load model %q: %w", path, err))
				continue
			}
			id := strings.TrimSpace(loaded.ID)
			name := strings.TrimSpace(loaded.DisplayName)
			if id == "" || name == "" {
				continue
			}
			key := strings.ToLower(id)
			if previous, exists := seenIDs[key]; exists {
				problems = append(problems, fmt.Errorf("duplicate model id %q in %q and %q", id, previous, path))
				continue
			}
			seenIDs[key], seenPaths[strings.ToLower(path)] = path, true
			result = append(result, Model{
				ID: id, DisplayName: name, Description: loaded.Description, Path: path,
				Language: loaded.Language,
				Version:  loaded.Version, FeatureVersion: loaded.FeatureVersion, Mode: loaded.Mode,
				SHA256:           fmt.Sprintf("%x", sha256.Sum256(data)),
				DefaultPriority:  loaded.DefaultPriority,
				RequiresFeatures: loaded.RequiresExternalFeatures(), FrameContour: loaded.HasFrameContour(),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DefaultPriority != result[j].DefaultPriority {
			return result[i].DefaultPriority > result[j].DefaultPriority
		}
		return result[i].DisplayName < result[j].DisplayName
	})
	return result, errors.Join(problems...)
}

func (catalog *Catalog) Model(id string) (Model, bool) {
	for _, item := range catalog.Models {
		if item.ID == id || item.Path == id {
			return item, true
		}
	}
	return Model{}, false
}

func DefaultModelDirectories() []string {
	var executable, current string
	if path, err := os.Executable(); err == nil {
		executable = path
	}
	if path, err := os.Getwd(); err == nil {
		current = path
	}
	return defaultModelDirectories(executable, current)
}

func defaultModelDirectories(executable, current string) []string {
	var directories []string
	if executable != "" {
		root := filepath.Dir(executable)
		if strings.EqualFold(filepath.Base(root), "tools") || strings.EqualFold(filepath.Base(root), "app") {
			root = filepath.Dir(root)
		}
		if packaged := filepath.Join(root, "models"); isDirectory(packaged) {
			directories = append(directories, packaged)
		}
	}
	if current != "" && len(directories) == 0 {
		directories = append(directories, filepath.Join(workspaceRoot(current), "models"))
	}
	return uniqueDirectories(directories)
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func workspaceRoot(start string) string {
	current := filepath.Clean(start)
	for {
		if info, err := os.Stat(filepath.Join(current, "go.mod")); err == nil && !info.IsDir() {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return start
		}
		current = parent
	}
}

func uniqueDirectories(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		absolute, err := filepath.Abs(value)
		if err != nil {
			continue
		}
		key := strings.ToLower(filepath.Clean(absolute))
		if !seen[key] {
			seen[key] = true
			result = append(result, absolute)
		}
	}
	return result
}
