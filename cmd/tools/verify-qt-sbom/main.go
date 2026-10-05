// verify-qt-sbomはQt SBOMファイルを検証し、GUIパッケージ内の同梱FFmpegを拒否する。
// macOSとWindowsのビルドから同じ結果を作る。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var sbomModules = []string{"qtbase", "qtdeclarative", "qtmultimedia"}

var (
	sha256Pattern       = regexp.MustCompile(`^[0-9A-Fa-f]{64}$`)
	soSuffixPattern     = regexp.MustCompile(`\.so(?:\.\d+)*$`)
	binarySuffixPattern = regexp.MustCompile(`\.(?:dll|dylib)(?:\.\d+)*$`)
	ffmpegNamePattern   = regexp.MustCompile(`^(?:avcodec|avformat|avutil|swresample|swscale)[-._].+$`)
)

type qtModulePrefix struct {
	prefix string
	module string
}

type qtModuleMarker struct {
	marker string
	module string
}

var qtDLLModules = map[string]string{
	"qtcore":             "qtbase",
	"qtgui":              "qtbase",
	"qtnetwork":          "qtbase",
	"qtopengl":           "qtbase",
	"qtconcurrent":       "qtbase",
	"qtwidgets":          "qtbase",
	"qtprintsupport":     "qtbase",
	"qtqml":              "qtdeclarative",
	"qtqmlmeta":          "qtdeclarative",
	"qtqmlmodels":        "qtdeclarative",
	"qtqmlworkerscript":  "qtdeclarative",
	"qtquick":            "qtdeclarative",
	"qtquickcontrols2":   "qtdeclarative",
	"qtquicktemplates2":  "qtdeclarative",
	"qtmultimedia":       "qtmultimedia",
	"qtmultimediaquick":  "qtmultimedia",
	"qtsvg":              "qtsvg",
	"qt6core":            "qtbase",
	"qt6gui":             "qtbase",
	"qt6network":         "qtbase",
	"qt6opengl":          "qtbase",
	"qt6concurrent":      "qtbase",
	"qt6widgets":         "qtbase",
	"qt6printsupport":    "qtbase",
	"qt6qml":             "qtdeclarative",
	"qt6qmlmeta":         "qtdeclarative",
	"qt6qmlmodels":       "qtdeclarative",
	"qt6qmlworkerscript": "qtdeclarative",
	"qt6quick":           "qtdeclarative",
	"qt6quickcontrols2":  "qtdeclarative",
	"qt6quicktemplates2": "qtdeclarative",
	"qt6multimedia":      "qtmultimedia",
	"qt6multimediaquick": "qtmultimedia",
	"qt6svg":             "qtsvg",
}

var qtPrefixModules = []qtModulePrefix{
	{"qt6multimedia", "qtmultimedia"},
	{"qtmultimedia", "qtmultimedia"},
	{"qt6svg", "qtsvg"},
	{"qtsvg", "qtsvg"},
	{"qt6qml", "qtdeclarative"},
	{"qtqml", "qtdeclarative"},
	{"qt6quick", "qtdeclarative"},
	{"qtquick", "qtdeclarative"},
	{"qt6labs", "qtdeclarative"},
	{"qtlabs", "qtdeclarative"},
	{"qt6core", "qtbase"},
	{"qtcore", "qtbase"},
	{"qt6gui", "qtbase"},
	{"qtgui", "qtbase"},
	{"qt6network", "qtbase"},
	{"qtnetwork", "qtbase"},
	{"qt6opengl", "qtbase"},
	{"qtopengl", "qtbase"},
	{"qtuiotouch", "qtbase"},
}

var qtPathModules = []qtModuleMarker{
	{"/qml/qtmultimedia/", "qtmultimedia"},
	{"/qml/qtqml/", "qtdeclarative"},
	{"/qml/qtquick/", "qtdeclarative"},
	{"/qml/qt/labs/", "qtdeclarative"},
	{"/multimedia/", "qtmultimedia"},
	{"/imageformats/", "qtbase"},
	{"/platforms/", "qtbase"},
	{"/generic/", "qtbase"},
	{"/networkinformation/", "qtbase"},
	{"/tls/", "qtbase"},
	{"/qml/", "qtdeclarative"},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verify-qt-sbom:", err)
		os.Exit(1)
	}
}

func run() error {
	packageRoot := flag.String("package-root", "", "package directory")
	sbomRoot := flag.String("sbom-root", "", "directory containing the raw Qt SPDX JSON files from the build audit")
	flag.Parse()
	if strings.TrimSpace(*packageRoot) == "" {
		return errors.New("--package-root is required")
	}
	root, err := filepath.Abs(*packageRoot)
	if err != nil {
		return err
	}
	explicitSBOMRoot := strings.TrimSpace(*sbomRoot) != ""
	sbomDir := ""
	if explicitSBOMRoot {
		if sbomDir, err = filepath.Abs(*sbomRoot); err != nil {
			return err
		}
	} else {
		sbomDir = filepath.Join(root, "licenses", "Qt", "sbom")
	}
	return verify(root, sbomDir, explicitSBOMRoot)
}

func verify(root, sbomDir string, explicitSBOMRoot bool) error {
	if info, err := os.Stat(sbomDir); err != nil || !info.IsDir() {
		hint := ""
		if !explicitSBOMRoot {
			hint = " (pass --sbom-root for an audit directory)"
		}
		return fmt.Errorf("Qt SBOM directory is missing: %s%s", sbomDir, hint)
	}
	files, err := filepath.Glob(filepath.Join(sbomDir, "*.spdx.json"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) != len(sbomModules) {
		return fmt.Errorf("expected %d Qt SBOM files, found %d", len(sbomModules), len(files))
	}
	type moduleSBOM struct {
		module string
		path   string
	}
	byModule := make([]moduleSBOM, 0, len(sbomModules))
	for _, module := range sbomModules {
		matches, err := filepath.Glob(filepath.Join(sbomDir, module+"-*.spdx.json"))
		if err != nil {
			return err
		}
		if len(matches) != 1 {
			return fmt.Errorf("expected one Qt SBOM for %s, found %d", module, len(matches))
		}
		byModule = append(byModule, moduleSBOM{module: module, path: matches[0]})
	}
	qtRoot := filepath.Join(root, "licenses", "Qt")
	manifestPath := filepath.Join(qtRoot, "Qt-SBOM-MANIFEST.txt")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("Qt SBOM manifest is missing: %s", manifestPath)
	}
	manifest := string(manifestBytes)
	for _, sbom := range byModule {
		expected, err := sha256File(sbom.path)
		if err != nil {
			return err
		}
		name := filepath.Base(sbom.path)
		pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(name) + `\s*$\n\s*SHA-256:\s*([0-9A-Fa-f]+)\s*$`)
		match := pattern.FindStringSubmatch(manifest)
		if match == nil || !sha256Pattern.MatchString(match[1]) || strings.ToUpper(match[1]) != expected {
			return fmt.Errorf("Qt SBOM hash is missing or incorrect: %s", name)
		}
		data, err := loadSBOM(sbom.path)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(stringValue(data["spdxVersion"]), "SPDX-") {
			return fmt.Errorf("Qt SBOM has no SPDX version: %s", name)
		}
		packages, ok := data["packages"].([]any)
		if !ok || len(packages) == 0 {
			return fmt.Errorf("Qt SBOM has no package list: %s", name)
		}
		if !strings.HasPrefix(stringValue(data["name"]), sbom.module+"-") {
			return fmt.Errorf("Qt SBOM document name does not match %s: %s", sbom.module, name)
		}
	}
	var multimedia map[string]any
	for _, sbom := range byModule {
		if sbom.module != "qtmultimedia" {
			continue
		}
		if multimedia, err = loadSBOM(sbom.path); err != nil {
			return err
		}
	}
	multimediaPackages, _ := multimedia["packages"].([]any)
	ffmpegCount := 0
	for _, item := range multimediaPackages {
		if object, ok := item.(map[string]any); ok && object["name"] == "FFmpeg" {
			ffmpegCount++
		}
	}
	if ffmpegCount != 1 {
		return errors.New("qtmultimedia SBOM does not identify exactly one FFmpeg package")
	}
	bundled, err := findBundledFFmpeg(root)
	if err != nil {
		return err
	}
	if len(bundled) > 0 {
		return fmt.Errorf("FFmpeg files must not be bundled: %s", strings.Join(bundled, ", "))
	}
	if info, err := os.Stat(filepath.Join(qtRoot, "FFmpeg-OPTIONAL.txt")); err != nil || info.IsDir() {
		return errors.New("FFmpeg-OPTIONAL.txt is missing")
	}
	for _, stale := range []string{"FFmpeg-SOURCE-AND-LICENSE.txt", "FFmpeg-SOURCE-OFFER.txt", "LGPL-2.1.txt"} {
		if _, err := os.Stat(filepath.Join(qtRoot, stale)); err == nil {
			return fmt.Errorf("obsolete bundled-FFmpeg license file is present: %s", stale)
		}
	}
	detected, err := detectModules(root)
	if err != nil {
		return err
	}
	if len(detected) == 0 {
		return errors.New("no Qt libraries were found in the package")
	}
	known := map[string]bool{}
	for _, sbom := range byModule {
		known[sbom.module] = true
	}
	var missing []string
	for module := range detected {
		if !known[module] {
			missing = append(missing, module)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("Qt libraries have no matching SBOM: %s", strings.Join(missing, ", "))
	}
	fmt.Printf("verified Qt SBOM: %s\n", sbomDir)
	fmt.Printf("modules: %s; FFmpeg bundled: no\n", strings.Join(sbomModules, ", "))
	return nil
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(hash.Sum(nil))), nil
}

func loadSBOM(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("invalid Qt SBOM JSON: %s: %w", path, err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("invalid Qt SBOM JSON: %s: %w", path, err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Qt SBOM must be an object: %s", path)
	}
	return object, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func ffmpegFile(name string) bool {
	lower := strings.ToLower(name)
	suffix := strings.ToLower(filepath.Ext(lower))
	if suffix != ".dll" && suffix != ".dylib" && !soSuffixPattern.MatchString(lower) {
		return false
	}
	lower = strings.TrimPrefix(lower, "lib")
	return strings.Contains(lower, "ffmpeg") || ffmpegNamePattern.MatchString(lower)
}

func normalizedBinaryStem(name string) string {
	stem := strings.ToLower(name)
	stem = binarySuffixPattern.ReplaceAllString(stem, "")
	stem = soSuffixPattern.ReplaceAllString(stem, "")
	return strings.TrimPrefix(stem, "lib")
}

func qtModuleForStem(stem string) string {
	if module, ok := qtDLLModules[stem]; ok {
		return module
	}
	for _, entry := range qtPrefixModules {
		if strings.HasPrefix(stem, entry.prefix) {
			return entry.module
		}
	}
	return ""
}

func qtModuleForPath(root, path, stem string) string {
	relative := ""
	if rel, err := filepath.Rel(root, path); err == nil {
		relative = "/" + strings.ToLower(filepath.ToSlash(rel)) + "/"
	}
	if strings.HasPrefix(stem, "qsvg") && (strings.Contains(relative, "/imageformats/") || strings.Contains(relative, "/iconengines/")) {
		return "qtsvg"
	}
	if strings.Contains(relative, "/iconengines/") {
		return "qtbase"
	}
	for _, entry := range qtPathModules {
		if strings.Contains(relative, entry.marker) {
			return entry.module
		}
	}
	return qtModuleForStem(stem)
}

func findBundledFFmpeg(root string) ([]string, error) {
	var bundled []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !ffmpegFile(entry.Name()) {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		bundled = append(bundled, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(bundled)
	return bundled, nil
}

func detectModules(root string) (map[string]bool, error) {
	detected := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		suffix := strings.ToLower(filepath.Ext(path))
		if suffix != ".dll" && suffix != ".dylib" && suffix != ".so" && !isFrameworkBinary(root, path) {
			return nil
		}
		stem := normalizedBinaryStem(entry.Name())
		module := qtModuleForPath(root, path, stem)
		if module == "" {
			if strings.HasPrefix(stem, "qt") {
				return fmt.Errorf("Qt library has no matching SBOM module: %s", entry.Name())
			}
			return nil
		}
		detected[module] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return detected, nil
}

func isFrameworkBinary(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		if part == ".framework" {
			return true
		}
	}
	return false
}
