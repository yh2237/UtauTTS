package engine

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"

	"utautts/internal/plugin"
)

// PublicIDはプロジェクト保存・UI表示・CLI／HTTP APIで使う安定識別子。
type PublicID string

// ProviderIDは実装を識別する。複数の定義が同じ実装を選べるためPublicIDとは分ける。
type ProviderID string

type Contract string

const (
	ContractUnknown           Contract = ""
	ContractUnitRenderer      Contract = "unit-renderer"
	ContractNeuralSynthesizer Contract = "neural-synthesizer"
)

// Capabilitiesはエンジン定義やproviderが公開する機能。定義はproviderの一部だけを公開してもよい。
type Capabilities = plugin.Capabilities

type ResourceKey string

const (
	ResourceWorldlineBridge    ResourceKey = "worldline_bridge"
	ResourceWorldEngine        ResourceKey = "world_engine"
	ResourceDiffSingerBridge   ResourceKey = "diffsinger_bridge"
	ResourceClassicResampler   ResourceKey = "classic_resampler"
	ResourceClassicWavtool     ResourceKey = "classic_wavtool"
	ResourceProviderExecutable ResourceKey = "provider_executable"
)

type ResourceRequirement struct {
	Key        ResourceKey
	Required   bool
	Executable bool
}

// Definitionはユーザーに見える合成エンジン宣言。renderer manifestから読む場合、Resourcesは解決済み絶対パスになる。
type Definition struct {
	ID              PublicID
	DisplayName     string
	Description     string
	Contract        Contract
	ContractVersion int
	Provider        ProviderID
	ProviderVersion string
	Protocol        string
	ProtocolVersion int
	ProviderArgs    []string
	ManifestVersion int
	Experimental    bool
	Acceleration    string
	DefaultPriority int
	Capabilities    Capabilities
	Resources       map[ResourceKey]string
}

func (definition Definition) Resource(key ResourceKey) string {
	return definition.Resources[key]
}

type Provider struct {
	ID           ProviderID
	Contract     Contract
	Version      string
	Capabilities Capabilities
	Requirements []ResourceRequirement
	// ClassicToolsは合成ごとに選んだUTAUのresamplerとwavtoolを使うprovider。
	ClassicTools bool
}

type Registry struct {
	providers map[ProviderID]Provider
}

var (
	ErrDefinitionNotFound      = errors.New("synthesis engine definition not found")
	ErrProviderUnavailable     = errors.New("synthesis engine provider is unavailable")
	ErrContractMismatch        = errors.New("synthesis engine contract does not match provider")
	ErrProviderVersionMismatch = errors.New("synthesis engine provider version does not match definition")
	ErrCapabilityMismatch      = errors.New("synthesis engine capability does not match provider")
	ErrResourcesUnavailable    = errors.New("synthesis engine resources are unavailable")
)

// 登録後のレジストリは変更しない。
func NewRegistry(providers ...Provider) (Registry, error) {
	result := Registry{providers: make(map[ProviderID]Provider, len(providers))}
	for _, provider := range providers {
		provider.ID = ProviderID(strings.TrimSpace(string(provider.ID)))
		provider.Version = strings.TrimSpace(provider.Version)
		if provider.ID == "" {
			return Registry{}, errors.New("provider id is required")
		}
		if provider.Contract != ContractUnitRenderer && provider.Contract != ContractNeuralSynthesizer {
			return Registry{}, fmt.Errorf("provider %q has unsupported contract %q", provider.ID, provider.Contract)
		}
		requirements := make(map[ResourceKey]struct{}, len(provider.Requirements))
		for _, requirement := range provider.Requirements {
			if requirement.Key == "" {
				return Registry{}, fmt.Errorf("provider %q has a resource requirement without a key", provider.ID)
			}
			if _, exists := requirements[requirement.Key]; exists {
				return Registry{}, fmt.Errorf("provider %q has duplicate resource requirement %q", provider.ID, requirement.Key)
			}
			requirements[requirement.Key] = struct{}{}
		}
		if _, exists := result.providers[provider.ID]; exists {
			return Registry{}, fmt.Errorf("duplicate provider id %q", provider.ID)
		}
		result.providers[provider.ID] = cloneProvider(provider)
	}
	return result, nil
}

func mustRegistry(providers ...Provider) Registry {
	registry, err := NewRegistry(providers...)
	if err != nil {
		panic(err)
	}
	return registry
}

var builtinRegistry = mustRegistry(
	Provider{ID: "waveform", Contract: ContractUnitRenderer, Version: "1", Capabilities: Capabilities{FramePitch: true, BoundaryBridge: true}},
	Provider{
		ID: "utautts-world-phrase", Contract: ContractUnitRenderer, Version: "1", Capabilities: Capabilities{FramePitch: true},
		Requirements: []ResourceRequirement{
			{Key: ResourceWorldEngine, Required: true},
			{Key: ResourceWorldlineBridge, Required: true, Executable: true},
		},
	},
	Provider{ID: "utau-external-resampler", Contract: ContractUnitRenderer, Version: "1", Capabilities: Capabilities{FramePitch: true}, ClassicTools: true},
	Provider{
		ID: "diffsinger", Contract: ContractNeuralSynthesizer, Version: "1", Capabilities: Capabilities{FramePitch: true, InternalTiming: true},
		Requirements: []ResourceRequirement{
			{Key: ResourceDiffSingerBridge, Required: true, Executable: true},
		},
	},
)

func BuiltinRegistry() Registry {
	return builtinRegistry
}

func (registry Registry) Provider(id ProviderID) (Provider, bool) {
	provider, found := registry.providers[id]
	return cloneProvider(provider), found
}

func (registry Registry) Providers() []Provider {
	result := make([]Provider, 0, len(registry.providers))
	for _, provider := range registry.providers {
		result = append(result, cloneProvider(provider))
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

func cloneProvider(provider Provider) Provider {
	provider.Requirements = append([]ResourceRequirement(nil), provider.Requirements...)
	return provider
}

func (registry Registry) Supports(id ProviderID) bool {
	_, found := registry.Provider(id)
	return found
}

type ResolvedEngine struct {
	Definition   Definition
	Provider     Provider
	Availability Availability
}

func (resolved ResolvedEngine) PublicID() PublicID {
	return resolved.Definition.ID
}

func (resolved ResolvedEngine) Resource(key ResourceKey) string {
	return resolved.Definition.Resource(key)
}

type Availability struct {
	Available bool                `json:"available"`
	Issues    []AvailabilityIssue `json:"issues,omitempty"`
}

type AvailabilityIssue struct {
	Resource ResourceKey `json:"resource,omitempty"`
	Message  string      `json:"message"`
}

func (availability Availability) Error() string {
	if availability.Available {
		return ""
	}
	if len(availability.Issues) == 0 {
		return "provider is unavailable"
	}
	messages := make([]string, 0, len(availability.Issues))
	for _, issue := range availability.Issues {
		messages = append(messages, issue.Message)
	}
	return strings.Join(messages, "; ")
}

func (resolved ResolvedEngine) RequireAvailable() error {
	if resolved.Availability.Available {
		return nil
	}
	return fmt.Errorf("%w: renderer %q: %s", ErrResourcesUnavailable, resolved.Definition.ID, resolved.Availability.Error())
}

// ResolveOptionsはアプリ指定の資源パスをmanifest由来より優先し、同梱資源と同じ事前検査に載せる。
type ResolveOptions struct {
	ResourceOverrides map[ResourceKey]string
}

type Resolver struct {
	registry Registry
}

func NewResolver(registry Registry) Resolver {
	return Resolver{registry: registry}
}

// 空IDは既定を選ぶ。未知の明示IDはエラーにする。
func (resolver Resolver) Resolve(definitions []Definition, requested string) (ResolvedEngine, error) {
	return resolver.ResolveWithOptions(definitions, requested, ResolveOptions{})
}

// 資源不足でも解決結果を返し、合成前にUIで理由を表示できるようにする。
func (resolver Resolver) ResolveWithOptions(definitions []Definition, requested string, options ResolveOptions) (ResolvedEngine, error) {
	requestedID := PublicID(strings.TrimSpace(requested))
	var definition *Definition
	if requestedID == "" {
		if len(definitions) > 0 {
			definition = &definitions[0]
		}
	} else {
		for index := range definitions {
			if definitions[index].ID == requestedID {
				definition = &definitions[index]
				break
			}
		}
	}
	if definition == nil {
		return ResolvedEngine{}, fmt.Errorf("%w: renderer %q is not available; select an installed renderer", ErrDefinitionNotFound, requestedID)
	}
	provider, found := resolver.registry.Provider(definition.Provider)
	if !found {
		if definition.Protocol == "utautts-provider" {
			provider = externalProviderForDefinition(*definition)
			found = true
		}
	}
	if !found {
		return ResolvedEngine{}, fmt.Errorf("%w: renderer plugin %q requires unavailable provider %q", ErrProviderUnavailable, definition.ID, definition.Provider)
	}
	if definition.Contract == ContractUnknown || definition.Contract != provider.Contract {
		return ResolvedEngine{}, fmt.Errorf("%w: renderer plugin %q declares %q but provider %q implements %q", ErrContractMismatch, definition.ID, definition.Contract, provider.ID, provider.Contract)
	}
	if definition.ProviderVersion != "" && definition.ProviderVersion != provider.Version {
		return ResolvedEngine{}, fmt.Errorf("%w: renderer plugin %q requires provider %q version %q but installed version is %q", ErrProviderVersionMismatch, definition.ID, provider.ID, definition.ProviderVersion, provider.Version)
	}
	if !provider.Capabilities.Supports(definition.Capabilities) {
		return ResolvedEngine{}, fmt.Errorf("%w: renderer plugin %q advertises unsupported capabilities for provider %q", ErrCapabilityMismatch, definition.ID, provider.ID)
	}
	resolvedDefinition := cloneDefinition(*definition)
	applyResourceOverrides(&resolvedDefinition, options)
	return ResolvedEngine{
		Definition:   resolvedDefinition,
		Provider:     provider,
		Availability: evaluateAvailability(resolvedDefinition, provider),
	}, nil
}

// 外部実装はmanifestで指定し、セッション開始時のハンドシェイクで検証する。
func externalProviderForDefinition(definition Definition) Provider {
	return Provider{
		ID:           definition.Provider,
		Contract:     definition.Contract,
		Version:      definition.ProviderVersion,
		Capabilities: definition.Capabilities,
		Requirements: []ResourceRequirement{{
			Key:        ResourceProviderExecutable,
			Required:   true,
			Executable: true,
		}},
	}
}

func cloneDefinition(definition Definition) Definition {
	if len(definition.Resources) > 0 {
		resources := definition.Resources
		definition.Resources = make(map[ResourceKey]string, len(definition.Resources))
		for key, value := range resources {
			definition.Resources[key] = value
		}
	}
	definition.ProviderArgs = append([]string(nil), definition.ProviderArgs...)
	return definition
}

func applyResourceOverrides(definition *Definition, options ResolveOptions) {
	for key, value := range options.ResourceOverrides {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if definition.Resources == nil {
			definition.Resources = make(map[ResourceKey]string)
		}
		definition.Resources[key] = value
	}
}

func evaluateAvailability(definition Definition, provider Provider) Availability {
	return CheckResources(definition.Resources, provider.Requirements...)
}

func CheckResources(resources map[ResourceKey]string, requirements ...ResourceRequirement) Availability {
	// wasmはエンジンをプロセス内に持つため、外部資源を検査しない。
	if runtime.GOOS == "js" {
		return Availability{Available: true}
	}
	issues := make([]AvailabilityIssue, 0, len(requirements))
	for _, requirement := range requirements {
		path := strings.TrimSpace(resources[requirement.Key])
		if path == "" {
			if requirement.Required {
				issues = append(issues, AvailabilityIssue{
					Resource: requirement.Key,
					Message:  fmt.Sprintf("required resource %q is not declared", requirement.Key),
				})
			}
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			if requirement.Required {
				issues = append(issues, AvailabilityIssue{
					Resource: requirement.Key,
					Message:  fmt.Sprintf("required resource %q is unavailable at %q", requirement.Key, path),
				})
			}
			continue
		}
		if info.IsDir() {
			issues = append(issues, AvailabilityIssue{
				Resource: requirement.Key,
				Message:  fmt.Sprintf("resource %q must be a file, got directory %q", requirement.Key, path),
			})
			continue
		}
		if requirement.Executable && runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
			issues = append(issues, AvailabilityIssue{
				Resource: requirement.Key,
				Message:  fmt.Sprintf("resource %q is not executable at %q", requirement.Key, path),
			})
		}
	}
	return Availability{Available: len(issues) == 0, Issues: issues}
}

// 先頭が既定となるため、カタログ順を保つ。
func DefinitionsFromCatalog(catalog *plugin.Catalog) []Definition {
	if catalog == nil {
		return nil
	}
	result := make([]Definition, 0, len(catalog.Renderers))
	for _, renderer := range catalog.Renderers {
		result = append(result, DefinitionFromV2(renderer))
	}
	return result
}

// 資源パスはpluginで解決するまでmanifest基準の相対パス。
func DefinitionFromV2(renderer plugin.Renderer) Definition {
	resourceNames := make(map[string]struct{}, len(renderer.Resources)+len(renderer.PlatformResources))
	for name := range renderer.Resources {
		resourceNames[name] = struct{}{}
	}
	for _, resources := range renderer.PlatformResources {
		for name := range resources {
			resourceNames[name] = struct{}{}
		}
	}
	resources := make(map[ResourceKey]string, len(resourceNames))
	for name := range resourceNames {
		if resource := renderer.Resource(name); strings.TrimSpace(resource.Path) != "" {
			resources[ResourceKey(name)] = resource.Path
		}
	}
	if len(resources) == 0 {
		resources = nil
	}
	return Definition{
		ID:              PublicID(renderer.ID),
		DisplayName:     renderer.DisplayName,
		Description:     renderer.Description,
		Contract:        Contract(strings.TrimSpace(renderer.Contract)),
		ContractVersion: renderer.ContractVersion,
		Provider:        ProviderID(strings.TrimSpace(renderer.Provider)),
		ProviderVersion: strings.TrimSpace(renderer.ProviderVersion),
		Protocol:        strings.TrimSpace(renderer.Protocol),
		ProtocolVersion: renderer.ProtocolVersion,
		ProviderArgs:    append([]string(nil), renderer.ProviderArgs...),
		ManifestVersion: renderer.ManifestVersion,
		Experimental:    renderer.Experimental,
		Acceleration:    renderer.Acceleration,
		DefaultPriority: renderer.DefaultPriority,
		Capabilities:    renderer.Capabilities,
		Resources:       resources,
	}
}
