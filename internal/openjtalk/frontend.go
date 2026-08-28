package openjtalk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"utautts/internal/prosody"
)

type Config struct {
	HelperPath     string
	DictionaryPath string
}

type Analysis struct {
	Version  int                    `json:"version"`
	Reading  string                 `json:"reading"`
	Morae    []string               `json:"morae"`
	Features []prosody.FeatureFrame `json:"features"`
}

func Analyze(text string, cfg Config) (*Analysis, error) {
	return AnalyzeContext(context.Background(), text, cfg)
}

const helperTimeout = 2 * time.Minute

func AnalyzeContext(ctx context.Context, text string, cfg Config) (*Analysis, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("Open JTalk frontend canceled: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, helperTimeout)
	defer cancel()
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("Open JTalk input is empty")
	}
	helper, err := resolveHelper(cfg.HelperPath)
	if err != nil {
		return nil, err
	}
	dictionary, err := resolveDictionary(cfg.DictionaryPath)
	if err != nil {
		return nil, err
	}
	response, err := invokeFrontendHelper(ctx, helper, dictionary, text)
	if err != nil {
		return nil, fmt.Errorf("Open JTalk frontend failed (helper=%q, dictionary=%q, text=%q): %w",
			helper, dictionary, previewText(text), err)
	}
	var result Analysis
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("decode Open JTalk response: %w", err)
	}
	if result.Version != 1 || result.Reading == "" || len(result.Morae) == 0 || len(result.Features) != len(result.Morae) {
		return nil, fmt.Errorf("invalid Open JTalk response: version=%d morae=%d features=%d", result.Version, len(result.Morae), len(result.Features))
	}
	return &result, nil
}

func resolveHelper(explicit string) (string, error) {
	name := "utautts-openjtalk-features"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return resolveFile(explicit, name, []string{
		filepath.Join("runtime", name),
		filepath.Join("tools", "openjtalk-feature-bridge", "bin", name),
	})
}

func resolveDictionary(explicit string) (string, error) {
	const name = "open_jtalk_dic_utf_8-1.11"
	if explicit != "" {
		if info, err := os.Stat(explicit); err == nil && info.IsDir() {
			return filepath.Abs(explicit)
		}
		return "", fmt.Errorf("Open JTalk dictionary not found: %s", explicit)
	}
	for _, root := range searchRoots() {
		for _, relative := range []string{
			filepath.Join("runtime", name),
			filepath.Join(".tmp-openjtalk", "pyopenjtalk", name),
		} {
			candidate := filepath.Join(root, relative)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return filepath.Abs(candidate)
			}
		}
	}
	return "", fmt.Errorf("Open JTalk dictionary not found; expected runtime/%s", name)
}

func resolveFile(explicit, description string, relatives []string) (string, error) {
	if explicit != "" {
		if info, err := os.Stat(explicit); err == nil && !info.IsDir() {
			return filepath.Abs(explicit)
		}
		return "", fmt.Errorf("%s not found: %s", description, explicit)
	}
	for _, root := range searchRoots() {
		for _, relative := range relatives {
			candidate := filepath.Join(root, relative)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return filepath.Abs(candidate)
			}
		}
	}
	return "", fmt.Errorf("%s not found", description)
}

func previewText(text string) string {
	const maxRunes = 120
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "…"
}

func searchRoots() []string {
	var roots []string
	if executable, err := os.Executable(); err == nil {
		roots = append(roots, filepath.Dir(executable), filepath.Dir(filepath.Dir(executable)))
	}
	if current, err := os.Getwd(); err == nil {
		roots = append(roots, current)
	}
	return roots
}
