package plugin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type PackageAsset struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type RuntimeDependency struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type manifestV2 struct {
	Schema          string                             `json:"$schema,omitempty"`
	ManifestVersion int                                `json:"manifest_version"`
	Kind            string                             `json:"kind"`
	ID              string                             `json:"id"`
	DisplayName     string                             `json:"display_name"`
	Description     string                             `json:"description,omitempty"`
	Version         string                             `json:"version"`
	Backend         string                             `json:"backend"`
	ProtocolVersion int                                `json:"protocol_version"`
	Experimental    bool                               `json:"experimental,omitempty"`
	DefaultPriority int                                `json:"default_priority,omitempty"`
	Runtimes        map[string]RuntimeDependency       `json:"runtimes,omitempty"`
	Platforms       map[string]map[string]PackageAsset `json:"platforms,omitempty"`
}

var packageID = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,95}$`)

func decodeRenderer(data []byte, directory string) (Renderer, error) {
	var version struct {
		ManifestVersion int `json:"manifest_version"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return Renderer{}, err
	}
	if version.ManifestVersion != 2 {
		var renderer Renderer
		err := json.Unmarshal(data, &renderer)
		renderer.BuiltIn = false
		return renderer, err
	}
	var doc manifestV2
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return Renderer{}, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return Renderer{}, fmt.Errorf("extra data after manifest")
	}
	if !packageID.MatchString(doc.ID) || doc.ID == "." || doc.ID == ".." || doc.Version == "" || doc.ProtocolVersion != 1 {
		return Renderer{}, fmt.Errorf("valid id, version and protocol_version=1 are required")
	}
	var backend *Renderer
	for _, item := range BuiltinRenderers() {
		if item.Backend == doc.Backend {
			copy := item
			backend = &copy
			break
		}
	}
	if backend == nil || doc.Backend == "utau-external-resampler" {
		return Renderer{}, fmt.Errorf("backend %q is not installed", doc.Backend)
	}
	renderer := Renderer{ManifestVersion: 2, Kind: doc.Kind, ID: doc.ID, DisplayName: doc.DisplayName, Description: doc.Description, Backend: doc.Backend, Version: doc.Version, Experimental: doc.Experimental, DefaultPriority: doc.DefaultPriority, Capabilities: backend.Capabilities, Acceleration: backend.Acceleration, Assets: map[string]string{}}
	for name, path := range backend.Assets {
		renderer.Assets[name] = path
	}
	for name, dependency := range doc.Runtimes {
		if _, ok := backend.Assets[name]; !ok {
			return Renderer{}, fmt.Errorf("unknown asset %q for backend", name)
		}
		path := runtimeAsset(dependency.ID)
		expected := map[string]string{"world_engine": "world-engine", "worldline_bridge": "worldline-bridge", "worldline": "worldline", "world_gpu": "waveform-gpu", "diffsinger_bridge": "diffsinger-bridge"}
		if dependency.Version != "1" || path == "" || expected[name] != dependency.ID {
			return Renderer{}, fmt.Errorf("unsupported runtime %s@%s", dependency.ID, dependency.Version)
		}
		renderer.Assets[name] = path
	}
	for platform, assets := range doc.Platforms {
		for name, asset := range assets {
			if _, ok := backend.Assets[name]; !ok {
				return Renderer{}, fmt.Errorf("unknown asset %q", name)
			}
			path, err := packagePath(directory, asset.Path)
			if err != nil {
				return Renderer{}, err
			}
			hash, err := hex.DecodeString(asset.SHA256)
			if err != nil || len(hash) != sha256.Size {
				return Renderer{}, fmt.Errorf("asset %q requires SHA-256", name)
			}
			if platform != runtime.GOOS+"-"+runtime.GOARCH && platform != "any" {
				continue
			}
			file, err := os.Open(path)
			if err != nil {
				return Renderer{}, err
			}
			hasher := sha256.New()
			_, copyErr := io.Copy(hasher, file)
			file.Close()
			if copyErr != nil {
				return Renderer{}, copyErr
			}
			if !bytes.Equal(hash, hasher.Sum(nil)) {
				return Renderer{}, fmt.Errorf("asset %q SHA-256 mismatch", name)
			}
		}
	}
	if len(doc.Platforms) > 0 {
		_, all := doc.Platforms["any"]
		_, local := doc.Platforms[runtime.GOOS+"-"+runtime.GOARCH]
		if !all && !local {
			return Renderer{}, fmt.Errorf("package does not support %s-%s", runtime.GOOS, runtime.GOARCH)
		}
	}
	for _, platform := range []string{"any", runtime.GOOS + "-" + runtime.GOARCH} {
		for name, asset := range doc.Platforms[platform] {
			renderer.Assets[name], _ = packagePath(directory, asset.Path)
		}
	}
	return renderer, nil
}

func packagePath(root, relative string) (string, error) {
	if relative == "" || strings.ContainsAny(relative, "\\:") || strings.HasPrefix(relative, "/") {
		return "", fmt.Errorf("invalid package path %q", relative)
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("package path escapes root: %q", relative)
	}
	return filepath.Join(root, clean), nil
}
