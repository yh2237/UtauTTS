// performance-baselineは代表ベンチとネイティブ合成の条件・結果を新しいディレクトリへ保存する。
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
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var benchPackages = []string{
	"./internal/oto", "./internal/audio", "./internal/pitch",
	"./internal/acoustic", "./internal/render/base",
}

type commandRecord struct {
	Command  []string `json:"command"`
	Log      string   `json:"log"`
	ExitCode int      `json:"exit_code"`
}

type runner struct {
	root     string
	out      string
	env      []string
	commands []commandRecord
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "performance-baseline:", err)
		os.Exit(1)
	}
}

func run() error {
	outFlag := flag.String("out", "", "new output directory")
	voicebankFlag := flag.String("voicebank", "", "voicebank directory")
	corpusFlag := flag.String("corpus", "tools/evaluation/japanese-v1.json", "corpus JSON")
	modelFlag := flag.String("model", "models/frame-intonation-tcn-v10.json", "prosody model JSON")
	count := flag.Int("count", 5, "benchmark count")
	repeat := flag.Int("repeat", 3, "synthesis repeat")
	benchtime := flag.String("benchtime", "200ms", "benchmark time")
	gomaxprocs := flag.Int("gomaxprocs", 4, "GOMAXPROCS")
	flag.Parse()
	if *outFlag == "" || *voicebankFlag == "" {
		return errors.New("--out and --voicebank are required")
	}
	if *count < 1 || *repeat < 1 || *gomaxprocs < 1 {
		return errors.New("count, repeat and gomaxprocs must be positive")
	}
	root, err := findRoot()
	if err != nil {
		return err
	}
	out := resolveAgainst(root, *outFlag)
	bank := resolveAgainst(root, *voicebankFlag)
	corpus := resolveAgainst(root, *corpusFlag)
	model := resolveAgainst(root, *modelFlag)
	for _, path := range []string{bank, corpus, model} {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("missing input: %s", path)
		}
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("output directory already exists: %s", out)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	env := append(os.Environ(), "GOMAXPROCS="+fmt.Sprint(*gomaxprocs))
	r := &runner{root: root, out: out, env: env}

	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	binary := filepath.Join(out, "tts-eval"+suffix)
	bridge := filepath.Join(out, "worldline-bridge"+suffix)
	metadata := map[string]any{
		"scope":         "native Japanese baseline, not GUI startup or browser",
		"platform":      platformString(),
		"processor":     processorString(),
		"logical_cpus":  runtime.NumCPU(),
		"gomaxprocs":    *gomaxprocs,
		"benchmark_cpu": 1,
		"arguments": map[string]any{
			"out": *outFlag, "voicebank": *voicebankFlag, "corpus": *corpusFlag, "model": *modelFlag,
			"count": *count, "repeat": *repeat, "benchtime": *benchtime, "gomaxprocs": *gomaxprocs,
		},
		"voicebank":     bank,
		"corpus_sha256": fileSHA256(corpus),
		"model_sha256":  fileSHA256(model),
	}
	commit, err := r.run([]string{"git", "rev-parse", "HEAD"}, "commit.txt", nil)
	if err != nil {
		return err
	}
	metadata["commit"] = strings.TrimSpace(commit)
	worktree, err := r.run([]string{"git", "status", "--porcelain"}, "worktree.txt", nil)
	if err != nil {
		return err
	}
	metadata["worktree"] = worktree
	goVersion, err := r.run([]string{"go", "version"}, "go-version.txt", nil)
	if err != nil {
		return err
	}
	metadata["go_version"] = strings.TrimSpace(goVersion)
	if _, err := r.run([]string{"git", "diff", "--binary"}, "source.patch", nil); err != nil {
		return err
	}
	modified, err := r.run([]string{"git", "diff", "--name-only", "--diff-filter=AM"}, "modified-files.txt", nil)
	if err != nil {
		return err
	}
	untracked, err := r.run([]string{"git", "ls-files", "--others", "--exclude-standard"}, "new-files.txt", nil)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, name := range append(strings.Split(modified, "\n"), strings.Split(untracked, "\n")...) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if err := copyFile(filepath.Join(root, name), filepath.Join(out, "_source", name)); err != nil {
			return err
		}
	}
	if err := writeJSON(filepath.Join(out, "metadata.json"), metadata); err != nil {
		return err
	}
	if _, err := r.run([]string{"go", "env", "-json", "GOOS", "GOARCH", "CGO_ENABLED", "GOFLAGS"}, "go-env.json", nil); err != nil {
		return err
	}
	if _, err := r.run([]string{"go", "list", "-m", "-json", "all"}, "modules.jsonl", nil); err != nil {
		return err
	}
	if _, err := r.run([]string{"go", "list", "-deps", "-json", "./cmd/utautts-cli"}, "cli-dependencies.jsonl", nil); err != nil {
		return err
	}
	assets, err := inputAssets(root, bank, suffix)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(out, "input-assets.json"), assets); err != nil {
		return err
	}
	benchArgs := []string{"go", "test", "-run", "^$", "-bench", ".", "-benchmem", "-cpu", "1",
		"-count", fmt.Sprint(*count), "-benchtime", *benchtime}
	benchArgs = append(benchArgs, benchPackages...)
	if _, err := r.run(benchArgs, "bench.txt", nil); err != nil {
		return err
	}
	if _, err := r.run([]string{"go", "build", "-o", binary, "./cmd/tools/tts-eval"}, "build-eval.txt", nil); err != nil {
		return err
	}
	if _, err := r.run([]string{"go", "build", "-o", bridge, "./cmd/utautts-worldline-bridge"}, "build-bridge.txt", nil); err != nil {
		return err
	}
	cli := filepath.Join(out, "utautts-cli"+suffix)
	wasm := filepath.Join(out, "utautts.wasm")
	if _, err := r.run([]string{"go", "build", "-o", cli, "./cmd/utautts-cli"}, "build-cli.txt", nil); err != nil {
		return err
	}
	wasmEnv := append(append([]string(nil), env...), "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
	if _, err := r.run([]string{"go", "build", "-o", wasm, "./cmd/utautts-wasm"}, "build-wasm.txt", wasmEnv); err != nil {
		return err
	}
	binaries := make([]map[string]any, 0, 4)
	for _, path := range []string{binary, bridge, cli, wasm} {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		binaries = append(binaries, map[string]any{"path": filepath.Base(path), "bytes": info.Size(), "sha256": fileSHA256(path)})
	}
	metadata["binaries"] = binaries
	if err := writeJSON(filepath.Join(out, "metadata.json"), metadata); err != nil {
		return err
	}
	command := []string{binary, "--voicebank", bank, "--corpus", corpus, "--model-file", model,
		"--bridge", bridge, "--repeat", fmt.Sprint(*repeat)}
	if _, err := r.run(append(append([]string(nil), command...), "--out", filepath.Join(out, "timing")), "timing.txt", nil); err != nil {
		return err
	}
	if _, err := r.run(append(append([]string(nil), command...), "--out", filepath.Join(out, "profile"), "--profile"), "profile.txt", nil); err != nil {
		return err
	}
	if _, err := r.run([]string{"go", "tool", "pprof", "-top", binary, filepath.Join(out, "profile", "cpu.pprof")}, "cpu-top.txt", nil); err != nil {
		return err
	}
	if _, err := r.run([]string{"go", "tool", "pprof", "-top", "-alloc_space", binary,
		filepath.Join(out, "profile", "allocs.pprof")}, "allocs-top.txt", nil); err != nil {
		return err
	}
	summary, err := summarize(filepath.Join(out, "timing", "report.json"))
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(out, "summary.json"), summary); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(out, "metadata.json"), metadata); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	fmt.Println("Saved", out)
	return nil
}

func (r *runner) run(command []string, log string, commandEnv []string) (string, error) {
	fmt.Println("RUN", displayCommand(command))
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Dir = r.root
	cmd.Env = commandEnv
	if cmd.Env == nil {
		cmd.Env = r.env
	}
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			exitCode = exit.ExitCode()
		} else {
			return "", err
		}
	}
	if err := os.WriteFile(filepath.Join(r.out, log), []byte(stdout.String()), 0o644); err != nil {
		return "", err
	}
	if stderr.Len() > 0 {
		if err := os.WriteFile(filepath.Join(r.out, log+".stderr"), []byte(stderr.String()), 0o644); err != nil {
			return "", err
		}
	}
	r.commands = append(r.commands, commandRecord{Command: command, Log: log, ExitCode: exitCode})
	if err := writeJSON(filepath.Join(r.out, "commands.json"), r.commands); err != nil {
		return "", err
	}
	if exitCode != 0 {
		fmt.Fprint(os.Stderr, stdout.String(), stderr.String())
		return "", fmt.Errorf("command failed; see %s", filepath.Join(r.out, log))
	}
	return stdout.String(), nil
}

func summarize(reportPath string) ([]map[string]any, error) {
	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, err
	}
	var report struct {
		Measurements []struct {
			ID         string  `json:"id"`
			ElapsedMS  float64 `json:"elapsed_ms"`
			Repetition int     `json:"repetition"`
			AudioMS    float64 `json:"audio_ms"`
		} `json:"Measurements"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	var order []string
	grouped := map[string][]struct {
		ElapsedMS  float64
		Repetition int
		AudioMS    float64
	}{}
	for _, row := range report.Measurements {
		if _, ok := grouped[row.ID]; !ok {
			order = append(order, row.ID)
		}
		grouped[row.ID] = append(grouped[row.ID], struct {
			ElapsedMS  float64
			Repetition int
			AudioMS    float64
		}{row.ElapsedMS, row.Repetition, row.AudioMS})
	}
	summary := make([]map[string]any, 0, len(order))
	for _, id := range order {
		cases := grouped[id]
		var warm []float64
		for _, row := range cases {
			if row.Repetition > 1 {
				warm = append(warm, row.ElapsedMS)
			}
		}
		entry := map[string]any{"id": id, "first_ms": cases[0].ElapsedMS, "audio_ms": cases[0].AudioMS}
		if len(warm) > 0 {
			entry["warm_median_ms"] = median(warm)
		} else {
			entry["warm_median_ms"] = nil
		}
		summary = append(summary, entry)
	}
	return summary, nil
}

func inputAssets(root, bank, suffix string) ([]map[string]any, error) {
	assets := []map[string]any{}
	err := filepath.WalkDir(bank, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(bank, path)
		if err != nil {
			return err
		}
		assets = append(assets, map[string]any{"path": filepath.ToSlash(relative),
			"bytes": info.Size(), "sha256": fileSHA256(path)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	runtimeMatches, err := filepath.Glob(filepath.Join(root, "runtime", "utautts-world-engine.*"))
	if err != nil {
		return nil, err
	}
	for _, path := range runtimeMatches {
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		assets = append(assets, map[string]any{"path": filepath.ToSlash(path),
			"bytes": info.Size(), "sha256": fileSHA256(path)})
	}
	helper := filepath.Join(root, "tools", "openjtalk-feature-bridge", "bin", "utautts-openjtalk-features"+suffix)
	if info, err := os.Stat(helper); err == nil && !info.IsDir() {
		assets = append(assets, map[string]any{"path": filepath.ToSlash(helper),
			"bytes": info.Size(), "sha256": fileSHA256(helper)})
	}
	return assets, nil
}

func findRoot() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		data, err := os.ReadFile(filepath.Join(current, "go.mod"))
		if err == nil && strings.Contains(string(data), "module utautts") {
			return current, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", errors.New("repository root was not found")
		}
		current = parent
	}
}

func resolveAgainst(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(root, path)
}

func displayCommand(command []string) string {
	parts := make([]string, len(command))
	for i, arg := range command {
		if strings.ContainsAny(arg, " \t\"") {
			parts[i] = `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}

func copyFile(source, destination string) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	return os.WriteFile(destination, data, 0o644)
}

func writeJSON(path string, value any) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func fileSHA256(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return ""
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func median(values []float64) float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func platformString() string {
	out, err := exec.Command("cmd", "/c", "ver").Output()
	if runtime.GOOS != "windows" || err != nil {
		return runtime.GOOS + "/" + runtime.GOARCH
	}
	return strings.TrimSpace(string(out))
}

func processorString() string {
	return os.Getenv("PROCESSOR_IDENTIFIER")
}
