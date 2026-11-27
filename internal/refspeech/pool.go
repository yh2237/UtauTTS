package refspeech

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yh2237/UtauIDX/speechpool"
	"github.com/yh2237/UtauTTS/internal/atomicfile"
)

func configuredPoolMode() speechpool.PoolMode {
	switch mode := speechpool.PoolMode(strings.TrimSpace(os.Getenv("UTAUTTS_POOL_MODE"))); mode {
	case speechpool.PoolBabbleOnly, speechpool.PoolMerge, speechpool.PoolRawOnly:
		return mode
	default:
		return speechpool.PoolAuto
	}
}

func configuredReversedPool() bool {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("UTAUTTS_REVERSED_POOL"))) {
	case "0", "false", "off", "no":
		return false
	default:
		return true
	}
}

const babbleSummaryName = "babble_summary.json"

var babbleMu sync.Mutex

func babbleDirPath(cacheDir, source string, external bool) string {
	babbleKey := "oto-babble-v1-" + source
	if external {
		babbleKey += "-external"
	}
	return filepath.Join(cacheDir, "babble", babbleKey)
}

func (s *Service) ensureBabbleEntries(ctx context.Context, root, color, source, worldPath string, files []speechpool.Entry, external bool) ([]speechpool.Entry, string, error) {
	babbleMu.Lock()
	defer babbleMu.Unlock()

	dir := babbleDirPath(s.cacheDir, source, external)
	path := filepath.Join(dir, babbleSummaryName)
	if data, err := os.ReadFile(path); err == nil {
		var summary speechpool.BabbleSummary
		if json.Unmarshal(data, &summary) == nil && hasBabbleFiles(summary, dir) {
			now := time.Now()
			_ = os.Chtimes(path, now, now)
			return speechpool.BabbleEntries(summary), dir, nil
		}
	}
	opts := speechpool.BabbleOptions{BankDir: root, WorldPath: worldPath, OutDir: dir, TargetSec: 180}
	for _, file := range files {
		opts.SourcePaths = append(opts.SourcePaths, file.Path)
	}
	summary, err := speechpool.GenerateBabble(ctx, opts)
	if err != nil {
		return nil, dir, fmt.Errorf("generate Oto-Babble: %w", err)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return nil, dir, err
	}
	if err := atomicfile.WriteFile(path, data); err != nil {
		return nil, dir, err
	}
	return speechpool.BabbleEntries(summary), dir, nil
}

func hasBabbleFiles(summary speechpool.BabbleSummary, dir string) bool {
	if len(summary.Files) == 0 || len(summary.Files) != summary.Phrases {
		return false
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for _, path := range summary.Files {
		resolved, err := filepath.Abs(path)
		if err != nil || filepath.Dir(resolved) != abs {
			return false
		}
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return false
		}
	}
	return true
}
