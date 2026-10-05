package native

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

type installVoicebankRequest struct {
	ZipPath string `json:"zip_path"`
	Name    string `json:"name,omitempty"`
}

func (e *Engine) installVoicebank(data []byte) (any, error) {
	var request installVoicebankRequest
	if len(data) != 0 {
		if err := json.Unmarshal(data, &request); err != nil {
			return nil, fmt.Errorf("decode installVoicebank request: %w", err)
		}
	}
	zipPath := strings.TrimSpace(request.ZipPath)
	if zipPath == "" {
		return nil, fmt.Errorf("zip_path is required")
	}
	archive, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("open voicebank archive: %w", err)
	}
	defer archive.Close()
	if len(archive.File) == 0 {
		return nil, fmt.Errorf("voicebank archive is empty")
	}

	if err := os.MkdirAll(e.config.VoiceDir, 0o755); err != nil {
		return nil, fmt.Errorf("create voice directory: %w", err)
	}
	staging, err := os.MkdirTemp(e.config.VoiceDir, ".install-")
	if err != nil {
		return nil, fmt.Errorf("create staging directory: %w", err)
	}
	defer os.RemoveAll(staging)

	for _, file := range archive.File {
		name := strings.Trim(strings.ReplaceAll(zipEntryName(file), "\\", "/"), "/")
		if name == "" {
			continue
		}
		relative, err := safeRelativePath(name)
		if err != nil {
			return nil, err
		}
		destination := filepath.Join(staging, relative)
		if !withinDirectory(staging, destination) {
			return nil, fmt.Errorf("archive entry escapes staging folder: %s", file.Name)
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return nil, fmt.Errorf("create folder %s: %w", destination, err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return nil, fmt.Errorf("create parent folder: %w", err)
		}
		if err := extractZipEntry(file, destination); err != nil {
			return nil, err
		}
	}

	bankRoot := findVoicebankRoot(staging)
	target := sanitizeDirectoryName(request.Name)
	if target == "" && bankRoot != staging {
		target = sanitizeDirectoryName(filepath.Base(bankRoot))
	}
	if target == "" {
		target = sanitizeDirectoryName(strings.TrimSuffix(filepath.Base(zipPath), filepath.Ext(zipPath)))
	}
	if target == "" {
		return nil, fmt.Errorf("could not determine voicebank folder name")
	}

	targetDir := filepath.Join(e.config.VoiceDir, target)
	if err := os.RemoveAll(targetDir); err != nil {
		return nil, fmt.Errorf("clear voicebank folder: %w", err)
	}
	if err := os.Rename(bankRoot, targetDir); err != nil {
		return nil, fmt.Errorf("place voicebank folder: %w", err)
	}

	if err := e.reload(); err != nil {
		return nil, err
	}
	return map[string]any{"installed": target, "voicebanks": e.voicebankList()}, nil
}

func findVoicebankRoot(root string) string {
	best := ""
	bestDepth := -1
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		switch strings.ToLower(info.Name()) {
		case "oto.ini", "dsconfig.yaml", "character.txt":
		default:
			return nil
		}
		dir := filepath.Dir(path)
		depth := 1
		if relative, relErr := filepath.Rel(root, dir); relErr == nil && relative != "." {
			depth = strings.Count(filepath.ToSlash(relative), "/") + 1
		}
		if best == "" || depth < bestDepth {
			best = dir
			bestDepth = depth
		}
		return nil
	})
	if best == "" {
		return root
	}
	return best
}

// UTAU音源ZIPの慣習に合わせ、UTF-8フラグがなければShift_JISとして復号する。
func zipEntryName(file *zip.File) string {
	name := file.Name
	if file.NonUTF8 {
		if decoded, _, err := transform.String(japanese.ShiftJIS.NewDecoder(), name); err == nil {
			name = decoded
		}
	}
	return name
}

func sanitizeDirectoryName(value string) string {
	cleaned := strings.Trim(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"), "/")
	if cleaned == "" {
		return ""
	}
	if strings.Contains(cleaned, "/") {
		cleaned = filepath.Base(cleaned)
	}
	if cleaned == "." || cleaned == ".." {
		return ""
	}
	return cleaned
}

func safeRelativePath(name string) (string, error) {
	cleaned := filepath.Clean(filepath.FromSlash(name))
	if cleaned == "." || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("invalid archive entry: %s", name)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("archive entry escapes voicebank folder: %s", name)
	}
	return cleaned, nil
}

func withinDirectory(root, target string) bool {
	rootAbsolute, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	targetAbsolute, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbsolute, targetAbsolute)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func extractZipEntry(file *zip.File, destination string) error {
	source, err := file.Open()
	if err != nil {
		return fmt.Errorf("open entry %s: %w", file.Name, err)
	}
	defer source.Close()
	target, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("create %s: %w", destination, err)
	}
	defer target.Close()
	if _, err := io.Copy(target, source); err != nil {
		return fmt.Errorf("write %s: %w", destination, err)
	}
	return nil
}
