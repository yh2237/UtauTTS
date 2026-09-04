package voicebank

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"utautts/internal/diffsinger"
)

// Summaryはvoicebankピッカーに表示するための軽量な情報。
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
		if summary, err := InspectSinger(root); err == nil {
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
	resolved, err := findVoicebankRoot(root)
	if err != nil {
		return Summary{}, err
	}
	return InspectSinger(resolved)
}

func findVoicebankRoot(root string) (string, error) {
	if hasDirectOto(root) || hasVoicebankMetadata(root) {
		if _, err := InspectSinger(root); err == nil {
			return root, nil
		}
	}
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
			if hasDirectOto(candidate) || (diffsinger.IsSinger(candidate) && hasVoicebankMetadata(candidate)) {
				preferred = append(preferred, candidate)
			}
		}
		if len(preferred) == 1 {
			return preferred[0], nil
		}
		// 複数サブバンクは共通の親を音源ルートにする。
		return root, nil
	}
	if diffsinger.IsSinger(root) {
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
	name := filepath.Base(absRoot)
	characterPath := findRootFile(absRoot, "character.txt")
	imagePath := ""
	if characterPath != "" {
		path := characterPath
		if text, readErr := readMetadata(path); readErr == nil {
			for _, line := range strings.Split(text, "\n") {
				parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
				if len(parts) != 2 {
					continue
				}
				key, configured := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
				switch {
				case strings.EqualFold(key, "name"):
					if configured != "" {
						name = configured
					}
				case strings.EqualFold(key, "image"):
					imagePath = safePresentationFile(absRoot, configured)
				}
			}
		}
	}
	if imagePath == "" {
		imagePath = findRootImage(absRoot)
	}
	readmePath := findRootFile(absRoot, "readme.txt")
	if readmePath == "" {
		readmePath = findRootFile(absRoot, "readme.md")
	}
	return Summary{Name: name, Path: absRoot, Kind: "utau", ImagePath: imagePath, CharacterPath: characterPath, ReadmePath: readmePath}, nil
}

// InspectSingerはUTAU音源とDiffSinger音源の表示情報を読む。
func InspectSinger(root string) (Summary, error) {
	if diffsinger.IsSinger(root) {
		return inspectDiffSinger(root)
	}
	return Inspect(root)
}

func inspectDiffSinger(root string) (Summary, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Summary{}, err
	}
	if !diffsinger.IsSinger(absRoot) {
		return Summary{}, ErrNoOto
	}
	name := filepath.Base(absRoot)
	characterPath := findRootFile(absRoot, "character.txt")
	imagePath := ""
	if characterPath != "" {
		if text, readErr := readMetadata(characterPath); readErr == nil {
			for _, line := range strings.Split(text, "\n") {
				parts := strings.SplitN(strings.TrimSpace(line), "=", 2)
				if len(parts) != 2 {
					continue
				}
				switch {
				case strings.EqualFold(strings.TrimSpace(parts[0]), "name") && strings.TrimSpace(parts[1]) != "":
					name = strings.TrimSpace(parts[1])
				case strings.EqualFold(strings.TrimSpace(parts[0]), "image"):
					imagePath = safePresentationFile(absRoot, strings.TrimSpace(parts[1]))
				}
			}
		}
	}
	if imagePath == "" {
		imagePath = findRootImage(absRoot)
	}
	readmePath := findRootFile(absRoot, "readme.txt")
	if readmePath == "" {
		readmePath = findRootFile(absRoot, "readme.md")
	}
	return Summary{Name: name, Path: absRoot, Kind: diffsinger.SingerKind, ImagePath: imagePath, CharacterPath: characterPath, ReadmePath: readmePath}, nil
}

func findRootImage(root string) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		if extension == ".bmp" || extension == ".png" || extension == ".jpg" || extension == ".jpeg" {
			return safePresentationFile(root, entry.Name())
		}
	}
	return ""
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
