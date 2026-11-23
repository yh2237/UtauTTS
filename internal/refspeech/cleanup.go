package refspeech

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/yh2237/UtauIDX/index"
)

// partRetentionは、使われていない部分キャッシュを残す期間。
// 録音1本ごとの解析結果なので、消しても索引の作成で作り直せる。
const partRetention = 30 * 24 * time.Hour

// 索引のキャッシュの上限。録音を変えると別の索引になるため、放っておくと増え続ける。
// 超えた分は、最後に使ったのが古いものから消す。
const (
	indexCacheMaxBytes int64 = 6 << 30
	indexCacheMaxFiles       = 24
)

type CleanupResult struct {
	Indexes int
	Parts   int
	Bytes   int64
}

// Summaryは削除の結果を1行にまとめる。何も消していなければ空を返す。
func (r CleanupResult) Summary() string {
	if r.Indexes == 0 && r.Parts == 0 {
		return ""
	}
	return fmt.Sprintf("索引のキャッシュを整理しました（使えない索引 %d 件、未使用の部分キャッシュ %d 件、%.1f MB）",
		r.Indexes, r.Parts, float64(r.Bytes)/(1<<20))
}

// Cleanupは、キャッシュにある使えない索引と、しばらく使われていない部分キャッシュを削除する。
func Cleanup(dir string) (CleanupResult, error) {
	var result CleanupResult
	if dir == "" {
		return result, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, err
	}
	for _, entry := range entries {
		ext := filepath.Ext(entry.Name())
		if !entry.Type().IsRegular() || (ext != ".idx" && ext != ".utsp") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if ext == ".utsp" {
			if header, err := index.ReadHeader(path); err == nil && header.Version == index.Version {
				continue
			}
		}
		if info, statErr := entry.Info(); statErr == nil {
			result.Indexes++
			result.Bytes += info.Size()
		}
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return result, fmt.Errorf("remove %s: %w", path, err)
		}
	}
	cutoff := time.Now().Add(-partRetention)
	partsDir := filepath.Join(dir, "parts")
	if info, err := os.Stat(partsDir); err == nil && info.IsDir() {
		count, bytes, err := removeUnusedParts(partsDir, cutoff)
		result.Parts, result.Bytes = count, result.Bytes+bytes
		if err != nil {
			return result, err
		}
	}
	// 上限を超えた分は、最後に使ったのが古いものから消す。
	count, bytes, err := removeUnusedBabble(filepath.Join(dir, "babble"), cutoff)
	result.Parts += count
	result.Bytes += bytes
	if err != nil {
		return result, err
	}
	trimmed, trimmedBytes := trimIndexCache(dir)
	result.Indexes += trimmed
	result.Bytes += trimmedBytes
	return result, nil
}

// trimIndexCacheは、索引の件数と大きさが上限を超えていれば、古いものから削除する。
func trimIndexCache(dir string) (int, int64) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0
	}
	type cachedIndex struct {
		path    string
		size    int64
		modTime time.Time
	}
	var indexes []cachedIndex
	var total int64
	for _, entry := range entries {
		if !entry.Type().IsRegular() || filepath.Ext(entry.Name()) != ".utsp" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		indexes = append(indexes, cachedIndex{
			path: filepath.Join(dir, entry.Name()), size: info.Size(), modTime: info.ModTime(),
		})
		total += info.Size()
	}
	if len(indexes) <= indexCacheMaxFiles && total <= indexCacheMaxBytes {
		return 0, 0
	}
	sort.Slice(indexes, func(i, j int) bool { return indexes[i].modTime.Before(indexes[j].modTime) })
	var count int
	var bytes int64
	for _, index := range indexes {
		if len(indexes)-count <= indexCacheMaxFiles && total <= indexCacheMaxBytes {
			break
		}
		if err := os.Remove(index.path); err != nil {
			// 使用中などで消せないものは残し、次回に回す。
			continue
		}
		count++
		bytes += index.size
		total -= index.size
	}
	return count, bytes
}

// removeUnusedPartsは、しばらく使われていない部分キャッシュを削除する。
func removeUnusedParts(dir string, cutoff time.Time) (int, int64, error) {
	var count int
	var bytes int64
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		switch filepath.Ext(entry.Name()) {
		case ".world", ".chan", ".part":
		default:
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil || info.ModTime().After(cutoff) {
			return nil
		}
		count++
		bytes += info.Size()
		if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
		return nil
	})
	if err != nil {
		return count, bytes, err
	}
	return count, bytes, nil
}

func removeUnusedBabble(dir string, cutoff time.Time) (int, int64, error) {
	babbleMu.Lock()
	defer babbleMu.Unlock()

	info, err := os.Lstat(dir)
	if os.IsNotExist(err) || (err == nil && !info.IsDir()) {
		return 0, 0, nil
	}
	if err != nil {
		return 0, 0, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, 0, err
	}
	var count int
	var bytes int64
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(filepath.Join(path, babbleSummaryName))
		if os.IsNotExist(err) {
			info, err = entry.Info()
		}
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		var size int64
		if err := filepath.WalkDir(path, func(_ string, ent os.DirEntry, err error) error {
			if err == nil && ent.Type().IsRegular() {
				if info, err := ent.Info(); err == nil {
					size += info.Size()
				}
			}
			return nil
		}); err != nil {
			return count, bytes, err
		}
		if err := os.RemoveAll(path); err != nil {
			return count, bytes, err
		}
		count++
		bytes += size
	}
	return count, bytes, nil
}
