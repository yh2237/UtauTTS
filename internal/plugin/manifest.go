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

func decodeRenderer(data []byte) (Renderer, error) {
	var doc Renderer
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
	if len(doc.Platforms) == 0 && len(doc.PlatformResources) > 0 {
		doc.Platforms = make([]string, 0, len(doc.PlatformResources))
		for platform := range doc.PlatformResources {
			doc.Platforms = append(doc.Platforms, platform)
		}
	}
	return doc, nil
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
		if setting.Type == "enum" && len(setting.Options) == 0 {
			return fmt.Errorf("renderer setting %q is enum without options", id)
		}
		if setting.Min != nil && setting.Max != nil && *setting.Min > *setting.Max {
			return fmt.Errorf("renderer setting %q has min greater than max", id)
		}
	}
	return nil
}
