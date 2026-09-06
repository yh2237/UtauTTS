package plugin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var rendererID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,95}$`)

type rendererManifest struct {
	Schema          string                       `json:"$schema,omitempty"`
	ManifestVersion int                          `json:"manifest_version"`
	Kind            string                       `json:"kind"`
	ID              string                       `json:"id"`
	DisplayName     string                       `json:"display_name"`
	Description     string                       `json:"description,omitempty"`
	Backend         string                       `json:"backend"`
	Version         string                       `json:"version,omitempty"`
	Experimental    bool                         `json:"experimental,omitempty"`
	Acceleration    string                       `json:"acceleration,omitempty"`
	DefaultPriority int                          `json:"default_priority,omitempty"`
	UpdateManaged   bool                         `json:"update_managed,omitempty"`
	Capabilities    Capabilities                 `json:"capabilities,omitempty"`
	Assets          map[string]string            `json:"assets,omitempty"`
	PlatformAssets  map[string]map[string]string `json:"platform_assets,omitempty"`
	Platforms       []string                     `json:"platforms,omitempty"`
}

type rendererManifestV2 struct {
	Schema            string                                 `json:"$schema,omitempty"`
	ManifestVersion   int                                    `json:"manifest_version"`
	Kind              string                                 `json:"kind"`
	ID                string                                 `json:"id"`
	DisplayName       string                                 `json:"display_name"`
	Description       string                                 `json:"description,omitempty"`
	Contract          string                                 `json:"contract"`
	ContractVersion   int                                    `json:"contract_version,omitempty"`
	Provider          string                                 `json:"provider"`
	ProviderVersion   string                                 `json:"provider_version"`
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
}

func decodeRenderer(data []byte, directory string) (Renderer, error) {
	var header struct {
		ManifestVersion int `json:"manifest_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Renderer{}, err
	}
	switch header.ManifestVersion {
	case 1:
		return decodeRendererV1(data)
	case 2:
		return decodeRendererV2(data, directory)
	default:
		return Renderer{}, fmt.Errorf("unsupported manifest_version %d", header.ManifestVersion)
	}
}

func decodeRendererV1(data []byte) (Renderer, error) {
	var doc rendererManifest
	if err := decodeStrictJSON(data, &doc); err != nil {
		return Renderer{}, err
	}
	if doc.ManifestVersion != 1 {
		return Renderer{}, fmt.Errorf("unsupported manifest_version %d", doc.ManifestVersion)
	}
	if err := validateRendererIDAndAssets(doc.ID, doc.Assets, doc.PlatformAssets); err != nil {
		return Renderer{}, err
	}
	platforms := append([]string(nil), doc.Platforms...)
	if len(platforms) == 0 && len(doc.PlatformAssets) > 0 {
		platforms = make([]string, 0, len(doc.PlatformAssets))
		for platform := range doc.PlatformAssets {
			platforms = append(platforms, platform)
		}
	}
	return Renderer{
		ManifestVersion: doc.ManifestVersion,
		Kind:            doc.Kind,
		ID:              doc.ID,
		DisplayName:     doc.DisplayName,
		Description:     doc.Description,
		Backend:         doc.Backend,
		Version:         doc.Version,
		Experimental:    doc.Experimental,
		Acceleration:    doc.Acceleration,
		DefaultPriority: doc.DefaultPriority,
		Capabilities:    doc.Capabilities,
		Assets:          doc.Assets,
		PlatformAssets:  doc.PlatformAssets,
		Platforms:       platforms,
	}, nil
}

func decodeRendererV2(data []byte, _ string) (Renderer, error) {
	var doc rendererManifestV2
	if err := decodeStrictJSON(data, &doc); err != nil {
		return Renderer{}, err
	}
	if doc.ManifestVersion != 2 {
		return Renderer{}, fmt.Errorf("unsupported manifest_version %d", doc.ManifestVersion)
	}
	if err := validateRendererIDAndResources(doc.ID, doc.Resources, doc.PlatformResources); err != nil {
		return Renderer{}, err
	}
	platforms := append([]string(nil), doc.Platforms...)
	if len(platforms) == 0 && len(doc.PlatformResources) > 0 {
		platforms = make([]string, 0, len(doc.PlatformResources))
		for platform := range doc.PlatformResources {
			platforms = append(platforms, platform)
		}
	}
	return Renderer{
		ManifestVersion:   doc.ManifestVersion,
		Kind:              doc.Kind,
		ID:                doc.ID,
		DisplayName:       doc.DisplayName,
		Description:       doc.Description,
		Contract:          doc.Contract,
		ContractVersion:   doc.ContractVersion,
		Provider:          doc.Provider,
		ProviderVersion:   doc.ProviderVersion,
		Protocol:          doc.Protocol,
		ProtocolVersion:   doc.ProtocolVersion,
		ProviderArgs:      append([]string(nil), doc.ProviderArgs...),
		Experimental:      doc.Experimental,
		Acceleration:      doc.Acceleration,
		DefaultPriority:   doc.DefaultPriority,
		Capabilities:      doc.Capabilities,
		Resources:         doc.Resources,
		PlatformResources: doc.PlatformResources,
		Platforms:         platforms,
	}, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fmt.Errorf("extra data after manifest")
	}
	return nil
}

func validateRendererIDAndAssets(id string, assets map[string]string, platformAssets map[string]map[string]string) error {
	if !rendererID.MatchString(id) || id == "." || id == ".." {
		return fmt.Errorf("invalid renderer id %q", id)
	}
	for name, value := range assets {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
			return fmt.Errorf("asset name and path must not be empty")
		}
	}
	for platform, assets := range platformAssets {
		if strings.TrimSpace(platform) == "" {
			return fmt.Errorf("platform name must not be empty")
		}
		for name, value := range assets {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
				return fmt.Errorf("platform asset name and path must not be empty")
			}
		}
	}
	return nil
}

func validateRendererIDAndResources(id string, resources map[string]RendererResource, platformResources map[string]map[string]RendererResource) error {
	if !rendererID.MatchString(id) || id == "." || id == ".." {
		return fmt.Errorf("invalid renderer id %q", id)
	}
	for name, resource := range resources {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(resource.Path) == "" {
			return fmt.Errorf("resource name and path must not be empty")
		}
	}
	for platform, resources := range platformResources {
		if strings.TrimSpace(platform) == "" {
			return fmt.Errorf("platform name must not be empty")
		}
		for name, resource := range resources {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(resource.Path) == "" {
				return fmt.Errorf("platform resource name and path must not be empty")
			}
		}
	}
	return nil
}
