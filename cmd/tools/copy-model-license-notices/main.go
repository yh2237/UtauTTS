// copy-model-license-noticesはモデルが参照するライセンス表記を検証してコピーする。
// license_noticesはパッケージ基準のPOSIXパスで、参照先はlicenses/内に限る。
// 実行ファイルに埋め込むモデル（safetensors）の表記は、__metadata__のlicense_notices（空白区切り）から読む。
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

var (
	errNoticeEmpty   = errors.New("license_notices entries must be nonempty strings")
	errNoticeInvalid = errors.New("license_notices entries must be normalized paths below licenses/")
)

type modelNotices struct {
	name    string
	notices []string
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "copy-model-license-notices:", err)
		os.Exit(1)
	}
}

func run() error {
	models := flag.String("models", "", "models directory")
	embedded := flag.String("embedded-models", "", "directory of safetensors models embedded in executables (optional)")
	packageRoot := flag.String("package-root", "", "package directory")
	repositoryRoot := flag.String("repository-root", "", "repository root")
	checkOnly := flag.Bool("check-only", false, "only verify that packaged notices exist")
	flag.Parse()
	if strings.TrimSpace(*models) == "" {
		return errors.New("--models is required")
	}
	if strings.TrimSpace(*packageRoot) == "" {
		return errors.New("--package-root is required")
	}
	modelsRoot, err := filepath.Abs(*models)
	if err != nil {
		return err
	}
	pkgRoot, err := filepath.Abs(*packageRoot)
	if err != nil {
		return err
	}
	repoRoot := ""
	if !*checkOnly {
		if strings.TrimSpace(*repositoryRoot) == "" {
			return errors.New("--repository-root is required unless --check-only is used")
		}
		if repoRoot, err = filepath.Abs(*repositoryRoot); err != nil {
			return err
		}
	}
	return copyNotices(modelsRoot, *embedded, pkgRoot, repoRoot, *checkOnly)
}

func copyNotices(modelsRoot, embeddedRoot, packageRoot, repositoryRoot string, checkOnly bool) error {
	notices, err := loadModels(modelsRoot)
	if err != nil {
		return err
	}
	if strings.TrimSpace(embeddedRoot) != "" {
		embedded, err := loadEmbeddedModels(embeddedRoot)
		if err != nil {
			return err
		}
		notices = append(notices, embedded...)
	}
	if checkOnly {
		for _, model := range notices {
			for _, relative := range model.notices {
				destination := filepath.Join(packageRoot, filepath.FromSlash(relative))
				if !isFile(destination) {
					return fmt.Errorf("%s: packaged license notice is missing: %s", model.name, relative)
				}
			}
		}
		return nil
	}
	licensesRoot, err := filepath.EvalSymlinks(filepath.Join(repositoryRoot, "licenses"))
	if err != nil {
		return fmt.Errorf("licenses directory was not found: %s", filepath.Join(repositoryRoot, "licenses"))
	}
	copied := map[string]bool{}
	var printed []string
	for _, model := range notices {
		for _, relative := range model.notices {
			source := filepath.Join(repositoryRoot, filepath.FromSlash(relative))
			resolved, err := filepath.EvalSymlinks(source)
			if err != nil {
				return fmt.Errorf("%s: license notice was not found: %s", model.name, relative)
			}
			inside, err := filepath.Rel(licensesRoot, resolved)
			if err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
				return fmt.Errorf("%s: license notice escapes licenses/: %s", model.name, relative)
			}
			if !isFile(resolved) {
				return fmt.Errorf("%s: license notice was not found: %s", model.name, relative)
			}
			destination := filepath.Join(packageRoot, filepath.FromSlash(relative))
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return err
			}
			if err := copyFilePreservingMetadata(resolved, destination); err != nil {
				return err
			}
			if !copied[relative] {
				copied[relative] = true
				printed = append(printed, relative)
			}
		}
	}
	sort.Strings(printed)
	for _, relative := range printed {
		fmt.Printf("model license notice: %s\n", relative)
	}
	return nil
}

func loadModels(modelsRoot string) ([]modelNotices, error) {
	paths, err := filepath.Glob(filepath.Join(modelsRoot, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no model JSON files found: %s", modelsRoot)
	}
	result := make([]modelNotices, 0, len(paths))
	for _, modelPath := range paths {
		name := filepath.Base(modelPath)
		data, err := os.ReadFile(modelPath)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid model JSON: %w", name, err)
		}
		var metadata map[string]any
		if err := json.Unmarshal(data, &metadata); err != nil || metadata == nil {
			if err == nil {
				err = errors.New("expected an object")
			}
			return nil, fmt.Errorf("%s: invalid model JSON: %w", name, err)
		}
		license, ok := metadata["license"].(string)
		if !ok || strings.TrimSpace(license) == "" {
			return nil, fmt.Errorf("%s: license is required", name)
		}
		notices, err := parseNotices(metadata, name)
		if err != nil {
			return nil, err
		}
		result = append(result, modelNotices{name: name, notices: notices})
	}
	return result, nil
}

func loadEmbeddedModels(root string) ([]modelNotices, error) {
	paths, err := filepath.Glob(filepath.Join(root, "*.safetensors"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return nil, fmt.Errorf("no embedded safetensors models found: %s", root)
	}
	result := make([]modelNotices, 0, len(paths))
	for _, modelPath := range paths {
		name := filepath.Base(modelPath)
		metadata, err := readSafeTensorsMetadata(modelPath)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		if strings.TrimSpace(metadata["license"]) == "" {
			return nil, fmt.Errorf("%s: license is required", name)
		}
		fields := strings.Fields(metadata["license_notices"])
		if len(fields) == 0 {
			return nil, fmt.Errorf("%s: license_notices is required", name)
		}
		notices := make([]string, 0, len(fields))
		for _, field := range fields {
			normalized, err := parseNoticePath(field)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			notices = append(notices, normalized)
		}
		result = append(result, modelNotices{name: name, notices: notices})
	}
	return result, nil
}

func readSafeTensorsMetadata(modelPath string) (map[string]string, error) {
	file, err := os.Open(modelPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var size uint64
	if err := binary.Read(file, binary.LittleEndian, &size); err != nil {
		return nil, fmt.Errorf("invalid safetensors header: %w", err)
	}
	if size == 0 || size > 1<<24 {
		return nil, fmt.Errorf("invalid safetensors header size %d", size)
	}
	header := make([]byte, size)
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, fmt.Errorf("invalid safetensors header: %w", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(header, &fields); err != nil {
		return nil, fmt.Errorf("invalid safetensors header: %w", err)
	}
	metadata := map[string]string{}
	if raw, ok := fields["__metadata__"]; ok {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, fmt.Errorf("invalid safetensors metadata: %w", err)
		}
	}
	return metadata, nil
}

func parseNotices(metadata map[string]any, modelName string) ([]string, error) {
	values, present := metadata["license_notices"]
	if !present || values == nil {
		legacy, legacyPresent := metadata["license_notice"]
		if !legacyPresent || legacy == nil {
			return nil, fmt.Errorf("%s: license_notices is required", modelName)
		}
		values = []any{legacy}
	}
	list, ok := values.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: license_notices must be an array", modelName)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%s: license_notices must not be empty", modelName)
	}
	notices := make([]string, 0, len(list))
	for _, value := range list {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("%s: %w", modelName, errNoticeEmpty)
		}
		normalized, err := parseNoticePath(text)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", modelName, err)
		}
		notices = append(notices, normalized)
	}
	return notices, nil
}

func parseNoticePath(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errNoticeEmpty
	}
	if trimmed != value || strings.HasPrefix(value, "/") || value != path.Clean(value) {
		return "", errNoticeInvalid
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 || parts[0] != "licenses" {
		return "", errNoticeInvalid
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, `\:`) {
			return "", errNoticeInvalid
		}
	}
	return value, nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func copyFilePreservingMetadata(source, destination string) error {
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.WriteFile(destination, data, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Chtimes(destination, info.ModTime(), info.ModTime())
}
