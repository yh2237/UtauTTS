package voicebank

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Summary struct {
	Name          string `json:"name"`
	Path          string `json:"path"`
	Kind          string `json:"kind,omitempty"`
	ImagePath     string `json:"image_path,omitempty"`
	CharacterPath string `json:"character_path,omitempty"`
	ReadmePath    string `json:"readme_path,omitempty"`
}

type Presentation struct {
	Summary       Summary
	CharacterText string
	ReadmeText    string
}

// Discoverはroot直下と二重展開された1階層内側から音源を探す。
func Discover(root string) ([]Summary, error) {
	if root == "" {
		root = "voice"
	}
	if looksLikeVoicebankRoot(root) {
		if summary, err := Inspect(root); err == nil {
			return []Summary{summary}, nil
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var result []Summary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		summary, inspectErr := inspectDiscoveredRoot(filepath.Join(root, entry.Name()))
		if inspectErr != nil {
			continue
		}
		result = append(result, summary)
	}
	if len(result) == 0 {
		return nil, ErrNoOto
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].Path < result[j].Path
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func inspectDiscoveredRoot(root string) (Summary, error) {
	if hasDirectOto(root) || hasVoicebankMetadata(root) {
		if summary, err := Inspect(root); err == nil {
			return summary, nil
		}
	}
	resolved, err := findNestedVoicebankRoot(root)
	if err != nil {
		return Summary{}, err
	}
	return Inspect(resolved)
}

func findVoicebankRoot(root string) (string, error) {
	if hasDirectOto(root) || hasVoicebankMetadata(root) {
		if _, err := Inspect(root); err == nil {
			return root, nil
		}
	}
	return findNestedVoicebankRoot(root)
}

func findNestedVoicebankRoot(root string) (string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if nested, nestedErr := findVoicebankRoot(filepath.Join(root, entry.Name())); nestedErr == nil {
			found = append(found, nested)
		}
	}
	if len(found) == 1 {
		return found[0], nil
	}
	if len(found) > 1 {
		var preferred []string
		for _, candidate := range found {
			if hasDirectOto(candidate) {
				preferred = append(preferred, candidate)
			}
		}
		if len(preferred) == 1 {
			return preferred[0], nil
		}
		// 複数サブバンクは共通の親を音源ルートにする。
		return root, nil
	}
	return "", ErrNoOto
}

func hasVoicebankMetadata(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if name == "character.txt" || name == "character.yaml" || name == "prefix.map" {
			return true
		}
	}
	return false
}

func hasDirectOto(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "oto.ini") {
			return true
		}
	}
	return false
}

// ResolveDirectoryは開発時の作業ディレクトリを優先し、次に実行ファイル基準で解決する。
func ResolveDirectory(configured string) string {
	if configured == "" {
		configured = "voice"
	}
	if filepath.IsAbs(configured) {
		return filepath.Clean(configured)
	}
	if absolute, err := filepath.Abs(configured); err == nil {
		if info, statErr := os.Stat(absolute); statErr == nil && info.IsDir() {
			return absolute
		}
	}
	if executable, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(executable), configured)
	}
	return configured
}

var errOtoFound = errors.New("oto.ini found")

// Inspectはoto.ini全体を解析せず、音源一覧に必要なメタデータだけを読む。
func Inspect(root string) (Summary, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Summary{}, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return Summary{}, err
	}
	if !info.IsDir() {
		if !strings.EqualFold(filepath.Base(absRoot), "oto.ini") {
			return Summary{}, ErrNoOto
		}
		absRoot = filepath.Dir(absRoot)
	}
	found := false
	err = filepath.WalkDir(absRoot, func(_ string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), "oto.ini") {
			found = true
			return errOtoFound
		}
		return nil
	})
	if err != nil && !errors.Is(err, errOtoFound) {
		return Summary{}, err
	}
	if !found {
		return Summary{}, ErrNoOto
	}
	characterPath := findRootFile(absRoot, "character.txt")
	name, imagePath := presentationIdentity(absRoot, characterPath)
	readmePath := findRootFile(absRoot, "readme.txt")
	if readmePath == "" {
		readmePath = findRootFile(absRoot, "readme.md")
	}
	return Summary{Name: name, Path: absRoot, Kind: "utau", ImagePath: imagePath, CharacterPath: characterPath, ReadmePath: readmePath}, nil
}

func presentationIdentity(root, characterPath string) (string, string) {
	name := filepath.Base(root)
	imagePath := ""
	if characterPath != "" {
		if text, err := readMetadata(characterPath); err == nil {
			for line := range strings.SplitSeq(text, "\n") {
				key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
				if !ok {
					continue
				}
				key, value = strings.TrimSpace(key), unquoteMetadataValue(strings.TrimSpace(value))
				switch {
				case strings.EqualFold(key, "name") && value != "":
					name = value
				case strings.EqualFold(key, "image") && imagePath == "":
					imagePath = safePresentationFile(root, value)
				}
			}
		}
	}
	if imagePath == "" {
		imagePath = characterYAMLImage(root)
	}
	if imagePath == "" {
		imagePath = findRootImage(root)
	}
	return name, imagePath
}

func characterYAMLImage(root string) string {
	path := findRootFile(root, "character.yaml")
	if path == "" {
		path = findRootFile(root, "character.yml")
	}
	if path == "" {
		return ""
	}
	text, err := readMetadata(path)
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(text, "\n") {
		if !strings.HasPrefix(line, "image:") {
			continue
		}
		if value := unquoteMetadataValue(strings.TrimSpace(strings.TrimPrefix(line, "image:"))); value != "" {
			return safePresentationFile(root, value)
		}
	}
	return ""
}

func unquoteMetadataValue(value string) string {
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1]
	}
	return value
}

func findRootImage(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	fallback := ""
	for _, entry := range entries {
		if entry.IsDir() || !isImageFile(entry.Name()) {
			continue
		}
		candidate := safePresentationFile(root, entry.Name())
		if candidate == "" {
			continue
		}
		if strings.EqualFold(entry.Name(), "icon.bmp") {
			return candidate
		}
		if fallback == "" {
			fallback = candidate
		}
	}
	return fallback
}

func isImageFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".bmp", ".png", ".jpg", ".jpeg":
		return true
	default:
		return false
	}
}

func safePresentationFile(root, relative string) string {
	if relative == "" || filepath.IsAbs(relative) {
		return ""
	}
	root = filepath.Clean(root)
	candidate := filepath.Clean(filepath.Join(root, filepath.FromSlash(relative)))
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	if info, err := os.Stat(candidate); err != nil || info.IsDir() {
		return ""
	}
	return candidate
}

func LoadPresentation(summary Summary) (Presentation, error) {
	result := Presentation{Summary: summary}
	var firstErr error
	if summary.CharacterPath != "" {
		result.CharacterText, firstErr = readMetadata(summary.CharacterPath)
	}
	if summary.ReadmePath != "" {
		text, err := readMetadata(summary.ReadmePath)
		if err != nil && firstErr == nil {
			firstErr = err
		} else if err == nil {
			result.ReadmeText = text
		}
	}
	return result, firstErr
}

func looksLikeVoicebankRoot(root string) bool {
	entries, err := os.ReadDir(root)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch {
		case strings.EqualFold(entry.Name(), "oto.ini"), strings.EqualFold(entry.Name(), "character.txt"), strings.EqualFold(entry.Name(), "prefix.map"), strings.EqualFold(entry.Name(), "dsconfig.yaml"):
			return true
		}
	}
	return false
}
