//go:build !js

package openjtalk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

func runFrontend(ctx context.Context, text string, cfg Config) (*Analysis, error) {
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
		return nil, fmt.Errorf("frontend helper failed (helper=%q, dictionary=%q): %w", helper, dictionary, err)
	}
	var result Analysis
	if err := json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("decode Open JTalk response: %w", err)
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
