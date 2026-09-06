package plugin

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"utautts/internal/prosody"
)

const ManifestVersion = 1

type Capabilities struct {
	FramePitch     bool `json:"frame_pitch,omitempty"`
	BoundaryBridge bool `json:"boundary_bridge,omitempty"`
}

type Renderer struct {
	ManifestVersion int               `json:"manifest_version"`
	Kind            string            `json:"kind"`
	ID              string            `json:"id"`
	DisplayName     string            `json:"display_name"`
	Description     string            `json:"description,omitempty"`
	Backend         string            `json:"backend"`
	Version         string            `json:"version,omitempty"`
	Experimental    bool              `json:"experimental,omitempty"`
	Acceleration    string            `json:"acceleration,omitempty"`
	DefaultPriority int               `json:"default_priority,omitempty"`
	Capabilities    Capabilities      `json:"capabilities,omitempty"`
	Assets          map[string]string `json:"assets,omitempty"`
	// PlatformAssetsはOS・CPU別のasset path。
	PlatformAssets map[string]map[string]string `json:"platform_assets,omitempty"`
	Platforms      []string                     `json:"platforms,omitempty"`
	Directory      string                       `json:"-"`
}

type Model struct {
	ID                   string          `json:"id"`
	DisplayName          string          `json:"display_name"`
	Description          string          `json:"description,omitempty"`
	Path                 string          `json:"path"`
	Version              int             `json:"version"`
	FeatureVersion       int             `json:"feature_version"`
	Mode                 string          `json:"mode"`
	SHA256               string          `json:"sha256"`
	Outputs              map[string]bool `json:"outputs,omitempty"`
	RecommendedRenderers []string        `json:"recommended_renderers,omitempty"`
	DefaultPriority      int             `json:"default_priority,omitempty"`
	RequiresFeatures     bool            `json:"requires_features,omitempty"`
	FrameContour         bool            `json:"frame_contour,omitempty"`
}

type Catalog struct {
	Problems   []string `json:"problems,omitempty"`
	Renderers  []Renderer
	Models     []Model
	Resamplers []ClassicTool
	Wavtools   []ClassicTool
}

type ClassicTool struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Path        string `json:"path,omitempty"`
	BuiltIn     bool   `json:"built_in,omitempty"`
}

func Discover(rendererDirectories, modelDirectories []string, supportsBackend func(string) bool) (*Catalog, error) {
	renderers, rendererErr := DiscoverRenderers(rendererDirectories, supportsBackend)
	models, modelErr := DiscoverModels(modelDirectories)
	return &Catalog{Renderers: renderers, Models: models}, errors.Join(rendererErr, modelErr)
}

// DiscoverWithDefaultsは明示ディレクトリを優先し、同梱の既定ディレクトリも探索する。
func DiscoverWithDefaults(rendererDirectories, modelDirectories []string, supportsBackend func(string) bool) (*Catalog, error) {
	defaultRendererDirs, defaultModelDirs := DefaultDirectories()
	rendererDirectories = append(rendererDirectories, defaultRendererDirs...)
	modelDirectories = append(modelDirectories, defaultModelDirs...)
	catalog, err := Discover(rendererDirectories, modelDirectories, supportsBackend)
	if err != nil {
		catalog.Problems = append(catalog.Problems, err.Error())
	}
	resamplerDirs, wavtoolDirs := DefaultClassicToolDirectories()
	catalog.Resamplers, catalog.Wavtools = DiscoverClassicTools(resamplerDirs, wavtoolDirs)
	catalog.Wavtools = append([]ClassicTool{{ID: "builtin", DisplayName: "UtauTTS built-in", BuiltIn: true}}, catalog.Wavtools...)
	sort.SliceStable(catalog.Renderers, func(i, j int) bool {
		return catalog.Renderers[i].DefaultPriority > catalog.Renderers[j].DefaultPriority
	})
	return catalog, nil
}

func DefaultClassicToolDirectories() (resamplerDirectories, wavtoolDirectories []string) {
	var roots []string
	if executable, err := os.Executable(); err == nil {
		root := filepath.Dir(executable)
		if strings.EqualFold(filepath.Base(root), "tools") || strings.EqualFold(filepath.Base(root), "app") {
			root = filepath.Dir(root)
		}
		roots = append(roots, root)
	}
	if current, err := os.Getwd(); err == nil {
		roots = append(roots, workspaceRoot(current))
	}
	for _, root := range uniqueDirectories(roots) {
		resamplerDirectories = append(resamplerDirectories, filepath.Join(root, "Resamplers"))
		wavtoolDirectories = append(wavtoolDirectories, filepath.Join(root, "Wavtools"))
	}
	return uniqueDirectories(resamplerDirectories), uniqueDirectories(wavtoolDirectories)
}

func DiscoverClassicTools(resamplerDirectories, wavtoolDirectories []string) ([]ClassicTool, []ClassicTool) {
	return discoverClassicTools(resamplerDirectories), discoverClassicTools(wavtoolDirectories)
}

func discoverClassicTools(directories []string) []ClassicTool {
	seen := map[string]bool{}
	var result []ClassicTool
	for _, root := range uniqueDirectories(directories) {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !isClassicExecutable(entry) {
				return nil
			}
			relative, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return nil
			}
			id := filepath.ToSlash(relative)
			key := strings.ToLower(id)
			if seen[key] {
				return nil
			}
			seen[key] = true
			result = append(result, ClassicTool{ID: id, DisplayName: strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())), Path: path})
			return nil
		})
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].ID) < strings.ToLower(result[j].ID) })
	return result
}

func isClassicExecutable(entry os.DirEntry) bool {
	extension := strings.ToLower(filepath.Ext(entry.Name()))
	if runtime.GOOS == "windows" {
		return extension == ".exe"
	}
	if extension != "" && extension != ".sh" {
		return false
	}
	info, err := entry.Info()
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

func (catalog *Catalog) Resampler(id string) (ClassicTool, bool) {
	return classicTool(catalog.Resamplers, id)
}

func (catalog *Catalog) Wavtool(id string) (ClassicTool, bool) {
	return classicTool(catalog.Wavtools, id)
}

func classicTool(values []ClassicTool, id string) (ClassicTool, bool) {
	for _, value := range values {
		if strings.EqualFold(value.ID, strings.TrimSpace(id)) {
			return value, true
		}
	}
	if id == "" && len(values) > 0 {
		return values[0], true
	}
	return ClassicTool{}, false
}

func DiscoverRenderers(directories []string, supportsBackend func(string) bool) ([]Renderer, error) {
	seen := map[string]string{}
	var result []Renderer
	var problems []error
	for _, root := range uniqueDirectories(directories) {
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				if !os.IsNotExist(walkErr) {
					problems = append(problems, fmt.Errorf("walk renderer directory %q: %w", root, walkErr))
				}
				return nil
			}
			if entry.IsDir() && path != root && strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.EqualFold(entry.Name(), "renderer.json") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				problems = append(problems, fmt.Errorf("read renderer manifest %q: %w", path, err))
				return nil
			}
			renderer, err := decodeRenderer(data, filepath.Dir(path))
			if err != nil {
				problems = append(problems, fmt.Errorf("decode renderer manifest %q: %w", path, err))
				return nil
			}
			if !rendererSupportedOnCurrentPlatform(renderer) {
				return nil
			}
			if err := validateRenderer(renderer, supportsBackend); err != nil {
				problems = append(problems, fmt.Errorf("renderer manifest %q: %w", path, err))
				return nil
			}
			key := strings.ToLower(renderer.ID)
			if _, exists := seen[key]; exists {
				// 明示ディレクトリを同梱定義より優先する。
				return nil
			}
			seen[key] = path
			renderer.Directory = filepath.Dir(path)
			result = append(result, renderer)
			return nil
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].DefaultPriority != result[j].DefaultPriority {
			return result[i].DefaultPriority > result[j].DefaultPriority
		}
		return result[i].DisplayName < result[j].DisplayName
	})
	return result, errors.Join(problems...)
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
				Version: loaded.Version, FeatureVersion: loaded.FeatureVersion, Mode: loaded.Mode,
				SHA256:               fmt.Sprintf("%x", sha256.Sum256(data)),
				Outputs:              cloneBoolMap(loaded.Outputs),
				RecommendedRenderers: append([]string(nil), loaded.RecommendedRenderers...),
				DefaultPriority:      loaded.DefaultPriority,
				RequiresFeatures:     loaded.RequiresExternalFeatures(), FrameContour: loaded.HasFrameContour(),
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

func cloneBoolMap(source map[string]bool) map[string]bool {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]bool, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}

func (catalog *Catalog) DefaultRenderer() string {
	if len(catalog.Renderers) == 0 {
		return ""
	}
	return catalog.Renderers[0].ID
}

func (catalog *Catalog) Renderer(id string) (Renderer, bool) {
	id = strings.TrimSpace(id)
	for _, item := range catalog.Renderers {
		if id != "" && item.ID == id {
			return item, true
		}
	}
	if id == "" && len(catalog.Renderers) > 0 {
		return catalog.Renderers[0], true
	}
	return Renderer{}, false
}

func (renderer Renderer) Asset(name string) string {
	value := strings.TrimSpace(renderer.Assets[name])
	for _, platform := range []string{runtime.GOOS + "-" + runtime.GOARCH, "any"} {
		if assets := renderer.PlatformAssets[platform]; assets != nil {
			if candidate := strings.TrimSpace(assets[name]); candidate != "" {
				value = candidate
				break
			}
		}
	}
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(renderer.Directory, value))
}

func (catalog *Catalog) Model(id string) (Model, bool) {
	for _, item := range catalog.Models {
		if item.ID == id || item.Path == id {
			return item, true
		}
	}
	return Model{}, false
}

func DefaultDirectories() (rendererDirectories, modelDirectories []string) {
	var executable, current string
	if path, err := os.Executable(); err == nil {
		executable = path
	}
	if path, err := os.Getwd(); err == nil {
		current = path
	}
	return defaultDirectories(executable, current)
}

func defaultDirectories(executable, current string) (rendererDirectories, modelDirectories []string) {
	var packagedRendererDirectory, packagedModelDirectory string
	if executable != "" {
		root := filepath.Dir(executable)
		if strings.EqualFold(filepath.Base(root), "tools") || strings.EqualFold(filepath.Base(root), "app") {
			root = filepath.Dir(root)
		}
		packagedRendererDirectory = filepath.Join(root, "renderer")
		packagedModelDirectory = filepath.Join(root, "models")
		if isDirectory(packagedRendererDirectory) {
			rendererDirectories = append(rendererDirectories, packagedRendererDirectory)
		}
		if isDirectory(packagedModelDirectory) {
			modelDirectories = append(modelDirectories, packagedModelDirectory)
		}
	}
	if current != "" {
		root := workspaceRoot(current)
		workspaceRendererDirectory := filepath.Join(root, "renderer")
		workspaceModelDirectory := filepath.Join(root, "models")
		if len(rendererDirectories) == 0 {
			rendererDirectories = append(rendererDirectories, workspaceRendererDirectory)
		}
		if len(modelDirectories) == 0 {
			modelDirectories = append(modelDirectories, workspaceModelDirectory)
		}
	}
	return uniqueDirectories(rendererDirectories), uniqueDirectories(modelDirectories)
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

func validateRenderer(renderer Renderer, supportsBackend func(string) bool) error {
	if renderer.ManifestVersion != ManifestVersion {
		return fmt.Errorf("unsupported manifest_version %d", renderer.ManifestVersion)
	}
	if renderer.Kind != "renderer" || renderer.ID == "" || renderer.DisplayName == "" || renderer.Backend == "" {
		return errors.New("kind=renderer, id, display_name, and backend are required")
	}
	if renderer.Acceleration != "" && renderer.Acceleration != "cpu" && renderer.Acceleration != "cuda" {
		return fmt.Errorf("unsupported acceleration %q", renderer.Acceleration)
	}
	if supportsBackend != nil && !supportsBackend(renderer.Backend) {
		return fmt.Errorf("backend %q is not installed", renderer.Backend)
	}
	return nil
}

func rendererSupportedOnCurrentPlatform(renderer Renderer) bool {
	platforms := renderer.Platforms
	if len(platforms) == 0 && len(renderer.PlatformAssets) > 0 {
		platforms = make([]string, 0, len(renderer.PlatformAssets))
		for platform := range renderer.PlatformAssets {
			platforms = append(platforms, platform)
		}
	}
	if len(platforms) == 0 {
		return true
	}
	current := runtime.GOOS + "-" + runtime.GOARCH
	for _, platform := range platforms {
		if strings.EqualFold(strings.TrimSpace(platform), "any") || strings.EqualFold(strings.TrimSpace(platform), current) {
			return true
		}
	}
	return false
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
