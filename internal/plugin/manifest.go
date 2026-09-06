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

func decodeRenderer(data []byte, _ string) (Renderer, error) {
	var doc rendererManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return Renderer{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return Renderer{}, fmt.Errorf("extra data after manifest")
	}
	if doc.ManifestVersion != ManifestVersion {
		return Renderer{}, fmt.Errorf("unsupported manifest_version %d", doc.ManifestVersion)
	}
	if !rendererID.MatchString(doc.ID) || doc.ID == "." || doc.ID == ".." {
		return Renderer{}, fmt.Errorf("invalid renderer id %q", doc.ID)
	}
	for name, value := range doc.Assets {
		if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
			return Renderer{}, fmt.Errorf("asset name and path must not be empty")
		}
	}
	for platform, assets := range doc.PlatformAssets {
		if strings.TrimSpace(platform) == "" {
			return Renderer{}, fmt.Errorf("platform name must not be empty")
		}
		for name, value := range assets {
			if strings.TrimSpace(name) == "" || strings.TrimSpace(value) == "" {
				return Renderer{}, fmt.Errorf("platform asset name and path must not be empty")
			}
		}
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
