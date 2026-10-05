// collect-go-licensesは配布パッケージのlicenses/Goを収集する。
// tools/go-license-modules.txtのモジュールについて、Go本体と各モジュールの
// ライセンス・通知をコピーする。bashとPowerShellの両方から同じ結果を作る。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "collect-go-licenses:", err)
		os.Exit(1)
	}
}

func run() error {
	packageDir := flag.String("package-dir", "", "package directory")
	root := flag.String("root", ".", "repository root")
	goCommand := flag.String("go", "go", "go command")
	modulesPath := flag.String("modules", "", "module list path (default tools/go-license-modules.txt)")
	flag.Parse()
	if strings.TrimSpace(*packageDir) == "" {
		return fmt.Errorf("--package-dir is required")
	}
	if *modulesPath == "" {
		*modulesPath = filepath.Join(*root, "tools", "go-license-modules.txt")
	}
	licenseDir := filepath.Join(*packageDir, "licenses", "Go")
	if err := os.MkdirAll(licenseDir, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(licenseDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			if err := os.Remove(filepath.Join(licenseDir, entry.Name())); err != nil {
				return err
			}
		}
	}

	goRoot, err := commandOutput(*goCommand, *root, "env", "GOROOT")
	if err != nil {
		return fmt.Errorf("go env GOROOT: %w", err)
	}
	goLicense := filepath.Join(licenseDir, "GO-LICENSE.txt")
	if err := copyRequired(filepath.Join(goRoot, "LICENSE"), goLicense); err != nil {
		return err
	}
	if err := copyRequired(filepath.Join(*root, "licenses", "Go", "CMUDICT-LICENSE.txt"), filepath.Join(licenseDir, "CMUDICT-LICENSE.txt")); err != nil {
		return err
	}
	if err := copyRequired(filepath.Join(*root, "licenses", "Go", "PINYIN-DATA-NOTICE.txt"), filepath.Join(licenseDir, "PINYIN-DATA-NOTICE.txt")); err != nil {
		return err
	}
	primaryHashes := map[string]bool{}
	if data, err := os.ReadFile(goLicense); err == nil {
		primaryHashes[contentHash(data)] = true
	}

	modules, err := readLines(*modulesPath)
	if err != nil {
		return err
	}
	for _, module := range modules {
		info, err := commandOutput(*goCommand, *root, "list", "-m", "-f", "{{.Dir}}|{{.Version}}", module)
		if err != nil {
			return fmt.Errorf("go list -m %s: %w", module, err)
		}
		moduleDir, moduleVersion, found := strings.Cut(info, "|")
		if !found || moduleDir == "" || moduleVersion == "" {
			return fmt.Errorf("could not resolve Go module metadata: %s", info)
		}
		safeName := strings.NewReplacer("/", "_", ".", "_").Replace(module)

		var sources []string
		err = filepath.WalkDir(moduleDir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || licenseFileKind(entry.Name()) == "" {
				return nil
			}
			sources = append(sources, path)
			return nil
		})
		if err != nil {
			return err
		}
		sort.Strings(sources)

		licenseCount := 0
		for _, source := range sources {
			primary := licenseFileKind(filepath.Base(source)) == "primary"
			relative, err := filepath.Rel(moduleDir, source)
			if err != nil {
				return err
			}
			destinationName := strings.NewReplacer("/", "__", "\\", "__").Replace(relative)
			switch relative {
			case "LICENSE":
				destinationName = "LICENSE.txt"
			case "NOTICE":
				destinationName = "NOTICE.txt"
			case "PATENTS":
				destinationName = "PATENTS.txt"
			}
			destination := filepath.Join(licenseDir, safeName+"-"+moduleVersion+"-"+destinationName)
			if primary {
				// 重複していてもprimaryの存在自体はモジュールの条件を満たす。
				licenseCount++
				data, err := os.ReadFile(source)
				if err != nil {
					return err
				}
				sum := contentHash(data)
				if primaryHashes[sum] {
					continue
				}
				if err := writeFile(destination, data); err != nil {
					return err
				}
				primaryHashes[sum] = true
				continue
			}
			if err := copyFile(source, destination); err != nil {
				return err
			}
		}
		if licenseCount == 0 {
			return fmt.Errorf("license file was not found for Go module: %s", module)
		}
	}
	return nil
}

// licenseFileKindはファイル名をprimary（LICENSE/COPYING）かother（通知類）に分類する。
func licenseFileKind(name string) string {
	upper := strings.ToUpper(name)
	primary := false
	switch {
	case upper == "LICENSE" || strings.HasPrefix(upper, "LICENSE."):
		primary = true
	case upper == "COPYING" || strings.HasPrefix(upper, "COPYING."):
		primary = true
	case strings.HasPrefix(upper, "THIRD_PARTY_NOTICES"), strings.HasPrefix(upper, "DATA_LICENSES"):
		return "other"
	case upper == "PATENTS" || strings.HasPrefix(upper, "PATENTS."):
		return "other"
	case upper == "NOTICE" || strings.HasPrefix(upper, "NOTICE."):
		return "other"
	}
	if primary {
		return "primary"
	}
	return ""
}

func commandOutput(command, dir string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, nil
}

func copyRequired(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("required Go license file was not found: %s", source)
	}
	return writeFile(destination, data)
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return writeFile(destination, data)
}

func writeFile(destination string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o644)
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
