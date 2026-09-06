// Package engine defines the boundary between a user-facing synthesis engine
// definition and the implementation that provides it.
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

// PublicID is the stable identifier stored by projects, exposed in the UI,
// and accepted by the CLI and HTTP API.
type PublicID string

// ProviderID identifies an implementation. It is intentionally distinct from
// PublicID: several definitions may select the same implementation.
type ProviderID string

// Contract describes the input a provider accepts.
type Contract string

const (
	ContractUnknown           Contract = ""
	ContractUnitRenderer      Contract = "unit-renderer"
	ContractNeuralSynthesizer Contract = "neural-synthesizer"
)

// Capabilities are the features exposed by an engine definition or provider.
// The v1 manifest adapter fills the definition from plugin.Capabilities; a
// definition may advertise only a subset of its provider's capabilities.
type Capabilities struct {
	FramePitch     bool
	BoundaryBridge bool
}

func (capabilities Capabilities) Supports(requested Capabilities) bool {
	return (!requested.FramePitch || capabilities.FramePitch) &&
		(!requested.BoundaryBridge || capabilities.BoundaryBridge)
}

// ResourceKey is the name of an engine runtime resource. The type is
// introduced now so v1 assets can be carried through the resolver without
// keeping their string map in tts.Config.
type ResourceKey string

const (
	ResourceWorldline          ResourceKey = "worldline"
	ResourceWorldlineBridge    ResourceKey = "worldline_bridge"
	ResourceWorldEngine        ResourceKey = "world_engine"
	ResourceWorldGPU           ResourceKey = "world_gpu"
	ResourceDiffSingerBridge   ResourceKey = "diffsinger_bridge"
	ResourceClassicResampler   ResourceKey = "classic_resampler"
	ResourceClassicWavtool     ResourceKey = "classic_wavtool"
	ResourceProviderExecutable ResourceKey = "provider_executable"
)

// ResourceRequirement declares a runtime dependency of a provider.
type ResourceRequirement struct {
	Key        ResourceKey
	Required   bool
	Executable bool
}

// Definition is the user-visible declaration of a synthesis engine.
// Resources are resolved absolute paths when the definition came from a v1
// renderer manifest.
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

// Resource returns the resolved path for a declared resource.
func (definition Definition) Resource(key ResourceKey) string {
	return definition.Resources[key]
}

// Provider describes an available implementation. In this first migration
// phase it is metadata only; the rendering interface will be added after the
// request resolver and typed resources are in place.
type Provider struct {
	ID           ProviderID
	Contract     Contract
	Version      string
	Capabilities Capabilities
	Requirements []ResourceRequirement
}

// Registry contains the implementations available in this binary.
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

// NewRegistry constructs an immutable provider registry.
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
		ID: "openutau-worldline-r-faithful", Contract: ContractUnitRenderer, Version: "1", Capabilities: Capabilities{FramePitch: true},
		Requirements: []ResourceRequirement{
			{Key: ResourceWorldline, Required: true},
			{Key: ResourceWorldlineBridge, Required: true, Executable: true},
		},
	},
	Provider{
		ID: "utautts-world-phrase", Contract: ContractUnitRenderer, Version: "1", Capabilities: Capabilities{FramePitch: true},
		Requirements: []ResourceRequirement{
			{Key: ResourceWorldEngine, Required: true},
			{Key: ResourceWorldlineBridge, Required: true, Executable: true},
		},
	},
	Provider{
		ID: "utautts-world-phrase-cuda", Contract: ContractUnitRenderer, Version: "1", Capabilities: Capabilities{FramePitch: true},
		Requirements: []ResourceRequirement{
			{Key: ResourceWorldEngine, Required: true},
			{Key: ResourceWorldlineBridge, Required: true, Executable: true},
			{Key: ResourceWorldGPU, Required: true},
		},
	},
	Provider{ID: "utau-external-resampler", Contract: ContractUnitRenderer, Version: "1", Capabilities: Capabilities{FramePitch: true}},
	Provider{
		ID: "diffsinger", Contract: ContractNeuralSynthesizer, Version: "1", Capabilities: Capabilities{FramePitch: true},
		Requirements: []ResourceRequirement{
			{Key: ResourceDiffSingerBridge, Required: true, Executable: true},
		},
	},
)

// BuiltinRegistry returns the providers bundled with the current binary.
func BuiltinRegistry() Registry {
	return builtinRegistry
}

// Provider returns a provider by implementation ID.
func (registry Registry) Provider(id ProviderID) (Provider, bool) {
	provider, found := registry.providers[id]
	return cloneProvider(provider), found
}

// Providers returns a stable copy suitable for diagnostics and tests.
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

// Supports reports whether the implementation is available in this binary.
func (registry Registry) Supports(id ProviderID) bool {
	_, found := registry.Provider(id)
	return found
}

// IsBuiltinProvider is a compatibility check used while renderer.json v1
// still calls its implementation field "backend".
func IsBuiltinProvider(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	return builtinRegistry.Supports(ProviderID(id))
}

// ResolvedEngine binds a user-visible definition to an available provider.
type ResolvedEngine struct {
	Definition   Definition
	Provider     Provider
	Availability Availability
}

// PublicID returns the stable ID selected by the caller.
func (resolved ResolvedEngine) PublicID() PublicID {
	return resolved.Definition.ID
}

// Resource returns a resolved runtime resource path.
func (resolved ResolvedEngine) Resource(key ResourceKey) string {
	return resolved.Definition.Resource(key)
}

// Availability is the preflight state of resources required by a provider.
type Availability struct {
	Available bool                `json:"available"`
	Issues    []AvailabilityIssue `json:"issues,omitempty"`
}

// AvailabilityIssue explains one unavailable resource.
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

// RequireAvailable returns a user-facing error when the selected definition
// cannot run with its current resources.
func (resolved ResolvedEngine) RequireAvailable() error {
	if resolved.Availability.Available {
		return nil
	}
	return fmt.Errorf("%w: renderer %q: %s", ErrResourcesUnavailable, resolved.Definition.ID, resolved.Availability.Error())
}

// ResolveOptions applies explicit application-level resource paths over
// manifest-derived paths. It lets --worldline and GUI settings participate in
// the same preflight as packaged runtime assets.
type ResolveOptions struct {
	ResourceOverrides map[ResourceKey]string
}

// Resolver resolves definitions against a registry of installed providers.
type Resolver struct {
	registry Registry
}

// NewResolver creates a resolver for one registry.
func NewResolver(registry Registry) Resolver {
	return Resolver{registry: registry}
}

// Resolve selects the requested definition. Only an empty requested ID may
// select the catalog default; a missing explicit ID remains an error.
func (resolver Resolver) Resolve(definitions []Definition, requested string) (ResolvedEngine, error) {
	return resolver.ResolveWithOptions(definitions, requested, ResolveOptions{})
}

// ResolveWithOptions resolves a definition and evaluates its required runtime
// resources. A definition may resolve successfully while Availability is false
// so UI callers can display the reason before synthesis is attempted.
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

// externalProviderForDefinition creates the small provider descriptor needed
// to resolve a manifest-declared process provider. Unlike built-in providers,
// its implementation is identified by the manifest and validated by the
// provider handshake when the session starts.
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

// CheckResources evaluates a resolved resource set. It is also used by
// provider-specific options such as Classic UTAU tool selection.
func CheckResources(resources map[ResourceKey]string, requirements ...ResourceRequirement) Availability {
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

// DefinitionsFromCatalog adapts all currently discovered renderer manifests.
// Catalog order is preserved so its first definition remains the default when
// the caller does not provide an ID.
func DefinitionsFromCatalog(catalog *plugin.Catalog) []Definition {
	if catalog == nil {
		return nil
	}
	result := make([]Definition, 0, len(catalog.Renderers))
	for _, renderer := range catalog.Renderers {
		if renderer.ManifestVersion == 2 {
			result = append(result, DefinitionFromV2(renderer))
		} else {
			result = append(result, DefinitionFromV1(renderer))
		}
	}
	return result
}

// DefinitionFromV1 adapts the existing renderer.json v1 shape. Its backend
// becomes a ProviderID and its contract is inferred. v1's version is the
// public definition version, not a provider version, so it is not used for
// provider compatibility checks.
func DefinitionFromV1(renderer plugin.Renderer) Definition {
	resourceNames := make(map[string]struct{}, len(renderer.Assets)+len(renderer.PlatformAssets))
	for name := range renderer.Assets {
		resourceNames[name] = struct{}{}
	}
	for _, assets := range renderer.PlatformAssets {
		for name := range assets {
			resourceNames[name] = struct{}{}
		}
	}
	resources := make(map[ResourceKey]string, len(resourceNames))
	for name := range resourceNames {
		if path := strings.TrimSpace(renderer.Asset(name)); path != "" {
			resources[ResourceKey(name)] = path
		}
	}
	if len(resources) == 0 {
		resources = nil
	}
	provider := ProviderID(strings.TrimSpace(renderer.Backend))
	return Definition{
		ID:              PublicID(renderer.ID),
		DisplayName:     renderer.DisplayName,
		Description:     renderer.Description,
		Contract:        ContractForLegacyProvider(provider),
		Provider:        provider,
		ManifestVersion: renderer.ManifestVersion,
		Experimental:    renderer.Experimental,
		Acceleration:    renderer.Acceleration,
		DefaultPriority: renderer.DefaultPriority,
		Capabilities: Capabilities{
			FramePitch:     renderer.Capabilities.FramePitch,
			BoundaryBridge: renderer.Capabilities.BoundaryBridge,
		},
		Resources: resources,
	}
}

// DefinitionFromV2 adapts the explicit contract/provider manifest shape.
// Resource paths remain relative to the manifest directory until plugin's
// resource resolver turns them into absolute paths.
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
		Capabilities: Capabilities{
			FramePitch:     renderer.Capabilities.FramePitch,
			BoundaryBridge: renderer.Capabilities.BoundaryBridge,
		},
		Resources: resources,
	}
}

// ContractForLegacyProvider maps the v1 backend vocabulary to the new input
// contracts. Unknown v1 backends remain unit renderers until a provider is
// registered for them; the resolver will then reject the unavailable provider.
func ContractForLegacyProvider(provider ProviderID) Contract {
	if provider == "diffsinger" {
		return ContractNeuralSynthesizer
	}
	if provider == "" {
		return ContractUnknown
	}
	return ContractUnitRenderer
}
