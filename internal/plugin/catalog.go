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

	"github.com/yh2237/UtauTTS/internal/prosody"
	"github.com/yh2237/UtauTTS/internal/settings"
)

const ManifestVersion = 2

type Capabilities struct {
	FramePitch bool `json:"frame_pitch,omitempty"`
	// レンダラー内でタイミングを扱う場合、日本語リズム補正を常に適用する。
	InternalTiming bool `json:"internal_timing,omitempty"`
	// 旧manifestを読めるよう残す。値は使わない。
	SpeechProsodyExperiment bool `json:"speech_prosody_experiment,omitempty"`
	BoundaryBridge          bool `json:"boundary_bridge,omitempty"`
}

// Supportsはproviderが必要とするcapabilityを満たすかを返す。
func (capabilities Capabilities) Supports(requested Capabilities) bool {
	return (!requested.FramePitch || capabilities.FramePitch) &&
		(!requested.InternalTiming || capabilities.InternalTiming)
}

type Renderer struct {
	Schema            string                                 `json:"$schema,omitempty"`
	ManifestVersion   int                                    `json:"manifest_version"`
	Kind              string                                 `json:"kind"`
	ID                string                                 `json:"id"`
	DisplayName       string                                 `json:"display_name"`
	Description       string                                 `json:"description,omitempty"`
	Contract          string                                 `json:"contract,omitempty"`
	ContractVersion   int                                    `json:"contract_version,omitempty"`
	Provider          string                                 `json:"provider,omitempty"`
	ProviderVersion   string                                 `json:"provider_version,omitempty"`
	Protocol          string                                 `json:"protocol,omitempty"`
	ProtocolVersion   int                                    `json:"protocol_version,omitempty"`
	ProviderArgs      []string                               `json:"provider_args,omitempty"`
	Experimental      bool                                   `json:"experimental,omitempty"`
	Acceleration      string                                 `json:"acceleration,omitempty"`
	DefaultPriority   int                                    `json:"default_priority,omitempty"`
	UpdateManaged     bool                                   `json:"update_managed,omitempty"`
	Capabilities      Capabilities                           `json:"capabilities,omitempty"`
	Resources         map[string]RendererResource            `json:"resources,omitempty"`
	PlatformResources map[string]map[string]RendererResource `json:"platform_resources,omitempty"`
	Platforms         []string                               `json:"platforms,omitempty"`
	Settings          []RendererSetting                      `json:"settings,omitempty"`
	Directory         string                                 `json:"-"`
}

type RendererSetting struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"`
	Group   string                 `json:"group,omitempty"`
	Default any                    `json:"default,omitempty"`
	Min     *float64               `json:"min,omitempty"`
	Max     *float64               `json:"max,omitempty"`
	Step    *float64               `json:"step,omitempty"`
	Label   string                 `json:"label,omitempty"`
	Options []RendererSettingValue `json:"options,omitempty"`
}

type RendererSettingValue struct {
	Value string `json:"value"`
	Label string `json:"label,omitempty"`
}

// 相対パスはrendererディレクトリ基準。資源の要否は実装側で検査する。
type RendererResource struct {
	Path       string `json:"path,omitempty"`
	Required   bool   `json:"required,omitempty"`
	Executable bool   `json:"executable,omitempty"`
}

type Model struct {
	ID                   string   `json:"id"`
	DisplayName          string   `json:"display_name"`
	Description          string   `json:"description,omitempty"`
	Language             string   `json:"language,omitempty"`
	Path                 string   `json:"path"`
	Version              int      `json:"version"`
	FeatureVersion       int      `json:"feature_version"`
	Mode                 string   `json:"mode"`
	SHA256               string   `json:"sha256"`
	RecommendedRenderers []string `json:"recommended_renderers,omitempty"`
	DefaultPriority      int      `json:"default_priority,omitempty"`
	RequiresFeatures     bool     `json:"requires_features,omitempty"`
	FrameContour         bool     `json:"frame_contour,omitempty"`
}

type Catalog struct {
	Problems  []string `json:"problems,omitempty"`
	Renderers []Renderer
	Models    []Model
}

func Discover(rendererDirectories, modelDirectories []string, supportsProvider func(string) bool) (*Catalog, error) {
	renderers, rendererErr := DiscoverRenderers(rendererDirectories, supportsProvider)
	models, modelErr := DiscoverModels(modelDirectories)
	return &Catalog{Renderers: renderers, Models: models}, errors.Join(rendererErr, modelErr)
}

// DiscoverWithDefaultsは明示ディレクトリを優先し、同梱の既定ディレクトリも探索する。
func DiscoverWithDefaults(rendererDirectories, modelDirectories []string, supportsProvider func(string) bool) (*Catalog, error) {
	defaultRendererDirs, defaultModelDirs := DefaultDirectories()
	rendererDirectories = append(rendererDirectories, defaultRendererDirs...)
	modelDirectories = append(modelDirectories, defaultModelDirs...)
	catalog, err := Discover(rendererDirectories, modelDirectories, supportsProvider)
	if err != nil {
		catalog.Problems = append(catalog.Problems, err.Error())
	}
	sort.SliceStable(catalog.Renderers, func(i, j int) bool {
		return catalog.Renderers[i].DefaultPriority > catalog.Renderers[j].DefaultPriority
	})
	return catalog, nil
}

func DiscoverRenderers(directories []string, supportsProvider func(string) bool) ([]Renderer, error) {
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
			renderer, err := decodeRenderer(data)
			if err != nil {
				problems = append(problems, fmt.Errorf("decode renderer manifest %q: %w", path, err))
				return nil
			}
			if !rendererSupportedOnCurrentPlatform(renderer) {
				return nil
			}
			if err := validateRenderer(renderer, supportsProvider); err != nil {
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
			result = append(result, withTableSettings(renderer))
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

// 設定表の順に並べ、manifestの同じIDで上書きし、固有項目を末尾に追加する。
func withTableSettings(renderer Renderer) Renderer {
	declared := make(map[string]RendererSetting, len(renderer.Settings))
	for _, setting := range renderer.Settings {
		declared[setting.ID] = setting
	}
	var merged []RendererSetting
	for _, setting := range settings.ForProvider(renderer.Provider) {
		if override, ok := declared[setting.ID]; ok {
			merged = append(merged, override)
			delete(declared, setting.ID)
			continue
		}
		merged = append(merged, rendererSettingFromTable(setting))
	}
	for _, setting := range renderer.Settings {
		if _, remaining := declared[setting.ID]; remaining {
			merged = append(merged, setting)
		}
	}
	renderer.Settings = merged
	return renderer
}

func rendererSettingFromTable(setting settings.Setting) RendererSetting {
	options := make([]RendererSettingValue, len(setting.Options))
	for index, option := range setting.Options {
		options[index] = RendererSettingValue{Value: option.Value, Label: option.Label}
	}
	if len(options) == 0 {
		options = nil
	}
	return RendererSetting{
		ID: setting.ID, Type: setting.Type, Group: setting.Group, Default: setting.Default,
		Min: setting.Min, Max: setting.Max, Step: setting.Step, Label: setting.Label,
		Options: options,
	}
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
				SHA256:               fmt.Sprintf("%x", sha256.Sum256(data)),
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

// 資源パスはレンダラーディレクトリ基準で解決する。
func (renderer Renderer) Resource(name string) RendererResource {
	resource := renderer.Resources[name]
	for _, platform := range []string{runtime.GOOS + "-" + runtime.GOARCH, "any"} {
		if resources := renderer.PlatformResources[platform]; resources != nil {
			if candidate, ok := resources[name]; ok {
				resource = candidate
				break
			}
		}
	}
	resource.Path = resolveRendererPath(renderer.Directory, resource.Path)
	return resource
}

func resolveRendererPath(directory, value string) string {
	value = strings.TrimSpace(value)
	if value == "" || filepath.IsAbs(value) {
		return value
	}
	return filepath.Clean(filepath.Join(directory, value))
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

func validateRenderer(renderer Renderer, supportsProvider func(string) bool) error {
	if renderer.ManifestVersion != ManifestVersion {
		return fmt.Errorf("unsupported manifest_version %d", renderer.ManifestVersion)
	}
	implementation := renderer.Provider
	if renderer.Kind != "synthesis-engine" || renderer.ID == "" || renderer.DisplayName == "" || renderer.Contract == "" || renderer.Provider == "" || renderer.ProviderVersion == "" {
		return errors.New("kind=synthesis-engine, id, display_name, contract, provider, and provider_version are required")
	}
	if renderer.Contract != "unit-renderer" && renderer.Contract != "neural-synthesizer" {
		return fmt.Errorf("unsupported contract %q", renderer.Contract)
	}
	if renderer.Protocol != "" {
		if renderer.Protocol != "utautts-provider" {
			return fmt.Errorf("unsupported provider protocol %q", renderer.Protocol)
		}
		if renderer.Contract != "unit-renderer" {
			return fmt.Errorf("external provider protocol currently supports only unit-renderer, got %q", renderer.Contract)
		}
		if renderer.ProtocolVersion != 1 {
			return fmt.Errorf("unsupported provider protocol version %d", renderer.ProtocolVersion)
		}
		if renderer.ContractVersion <= 0 {
			return errors.New("external provider contract_version must be positive")
		}
		resource := renderer.Resource("provider_executable")
		if strings.TrimSpace(resource.Path) == "" || !resource.Executable {
			return errors.New("external provider requires executable provider_executable resource")
		}
		for _, argument := range renderer.ProviderArgs {
			if strings.TrimSpace(argument) == "" {
				return errors.New("provider_args must not contain empty values")
			}
		}
	}
	if renderer.Acceleration != "" && renderer.Acceleration != "cpu" {
		return fmt.Errorf("unsupported acceleration %q", renderer.Acceleration)
	}
	if supportsProvider != nil && !supportsProvider(implementation) && renderer.Protocol == "" {
		return fmt.Errorf("provider %q is not installed", implementation)
	}
	return nil
}

func rendererSupportedOnCurrentPlatform(renderer Renderer) bool {
	platforms := renderer.Platforms
	if len(platforms) == 0 && len(renderer.PlatformResources) > 0 {
		platforms = make([]string, 0, len(renderer.PlatformResources))
		for platform := range renderer.PlatformResources {
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
