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
	Settings          []RendererSetting                      `json:"settings,omitempty"`
}

func decodeRenderer(data []byte, directory string) (Renderer, error) {
	var header struct {
		ManifestVersion int `json:"manifest_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Renderer{}, err
	}
	if header.ManifestVersion != 2 {
		return Renderer{}, fmt.Errorf("unsupported manifest_version %d", header.ManifestVersion)
	}
	return decodeRendererV2(data, directory)
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
	if err := validateRendererSettings(doc.Settings); err != nil {
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
		Settings:          append([]RendererSetting(nil), doc.Settings...),
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

// validateRendererSettingsは設定項目のid・type・範囲の整合性を検査する。
func validateRendererSettings(settings []RendererSetting) error {
	seen := make(map[string]struct{}, len(settings))
	for _, setting := range settings {
		id := strings.TrimSpace(setting.ID)
		if id == "" || id != setting.ID {
			return fmt.Errorf("renderer setting id must not be empty or padded: %q", setting.ID)
		}
		if _, exists := seen[id]; exists {
			return fmt.Errorf("duplicate renderer setting id %q", id)
		}
		seen[id] = struct{}{}
		switch setting.Type {
		case "integer", "number", "boolean", "enum", "string":
		default:
			return fmt.Errorf("renderer setting %q has unsupported type %q", id, setting.Type)
		}
		if setting.Type == "enum" && len(setting.Options) == 0 && setting.OptionsSource == "" {
			return fmt.Errorf("renderer setting %q is enum without options", id)
		}
		switch setting.OptionsSource {
		case "", "resamplers", "wavtools":
		default:
			return fmt.Errorf("renderer setting %q has unsupported options_source %q", id, setting.OptionsSource)
		}
		if setting.Min != nil && setting.Max != nil && *setting.Min > *setting.Max {
			return fmt.Errorf("renderer setting %q has min greater than max", id)
		}
	}
	return nil
}
