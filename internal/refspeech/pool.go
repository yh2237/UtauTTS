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

	"github.com/yh2237/UtauIDX/atomicfile"
	"github.com/yh2237/UtauIDX/index"
	"github.com/yh2237/UtauIDX/speechpool"
	"github.com/yh2237/audiodsp/wav"
)

func configuredPoolMode() speechpool.PoolMode {
	switch mode := speechpool.PoolMode(strings.TrimSpace(os.Getenv("UTAUTTS_POOL_MODE"))); mode {
	case speechpool.PoolBabbleOnly, speechpool.PoolMerge, speechpool.PoolRawOnly:
		return mode
	default:
		return speechpool.PoolAuto
	}
}

func estimateRawFrames(files []speechpool.Entry) int {
	var frames int
	for _, file := range files {
		pcm, err := wav.ReadFile(file.Path)
		if err != nil || pcm.Channels <= 0 || pcm.SampleRate <= 0 {
			continue
		}
		durationMS := float64(len(pcm.Data)/pcm.Channels) * 1000 / float64(pcm.SampleRate)
		if durationMS < index.MinimumFileSec*1000 {
			continue
		}
		start, end := index.RecordingSpan(file.Regions, durationMS)
		frames += int((end - start) / index.FrameMS)
	}
	return frames
}

const babbleSummaryName = "babble_summary.json"

var babbleMu sync.Mutex

func (s *Service) ensureBabbleEntries(ctx context.Context, root, color, source, worldPath string, files []speechpool.Entry, external bool) ([]speechpool.Entry, error) {
	babbleMu.Lock()
	defer babbleMu.Unlock()

	babbleKey := "oto-babble-v1-" + source
	if external {
		babbleKey += "-external"
	}
	dir := filepath.Join(s.cacheDir, "babble", babbleKey)
	path := filepath.Join(dir, babbleSummaryName)
	if data, err := os.ReadFile(path); err == nil {
		var summary speechpool.BabbleSummary
		if json.Unmarshal(data, &summary) == nil && hasBabbleFiles(summary, dir) {
			now := time.Now()
			_ = os.Chtimes(path, now, now)
			return speechpool.BabbleEntries(summary), nil
		}
	}
	opts := speechpool.BabbleOptions{BankDir: root, WorldPath: worldPath, OutDir: dir, TargetSec: 180}
	if color != "" || external {
		for _, file := range files {
			opts.SourcePaths = append(opts.SourcePaths, file.Path)
		}
	}
	summary, err := speechpool.GenerateBabble(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("generate Oto-Babble: %w", err)
	}
	data, err := json.Marshal(summary)
	if err != nil {
		return nil, err
	}
	if err := atomicfile.WriteFile(path, data); err != nil {
		return nil, err
	}
	return speechpool.BabbleEntries(summary), nil
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
