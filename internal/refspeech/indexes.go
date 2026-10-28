package refspeech

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yh2237/UtauIDX/refmatch"
)

// IndexInfoは保存済みの索引の情報。
type IndexInfo struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Voicebank  string `json:"voicebank"`
	Model      string `json:"model"`
	Version    int    `json:"version"`
	SizeBytes  int64  `json:"size_bytes"`
	ModifiedMS int64  `json:"modified_ms"`
}

// ListIndexesはヘッダだけを読み、保存済みの索引を名前順に返す。
func ListIndexes(dir string) ([]IndexInfo, error) {
	result := make([]IndexInfo, 0)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".idx") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		header, err := refmatch.ReadIndexInfo(path)
		if err != nil {
			continue
		}
		root := header.Source
		// 旧索引のSourceはハッシュ。録音の共通ディレクトリを表示に使う。
		if header.CacheKey == "" && isLegacySource(root) {
			root = ""
			for _, file := range header.Files {
				if root == "" {
					root = filepath.Dir(file)
					continue
				}
				for {
					rel, err := filepath.Rel(root, file)
					if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
						break
					}
					parent := filepath.Dir(root)
					if parent == root {
						root = ""
						break
					}
					root = parent
				}
			}
		}
		name := ""
		if root != "" {
			name = filepath.Base(filepath.Clean(root))
		}
		result = append(result, IndexInfo{Name: entry.Name(), Path: path, Voicebank: name, Model: header.Model, Version: header.Version, SizeBytes: header.SizeBytes, ModifiedMS: header.ModifiedMS})
	}
	return result, nil
}

func isLegacySource(source string) bool {
	if len(source) != 32 {
		return false
	}
	for _, c := range source {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func matchesSource(index *refmatch.Index, root, key string) bool {
	return index.Source == key || index.Source == filepath.Clean(root) && index.CacheKey == key
}

// PrepareStatusは索引の準備を開始し、完成を待たずに状態を返す。
func (s *Service) PrepareStatus(root string, recordings []refmatch.Recording, world string) IndexStatus {
	s.dropStale(root, recordings)
	entry := s.entry(root, recordings, world)
	s.mu.Lock()
	defer s.mu.Unlock()
	return entry.status
}

// DeleteIndexはキャッシュ内の索引を削除し、読み込み済みの索引も捨てる。
func DeleteIndex(dir, name string) (bool, error) {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\:`) || !strings.HasSuffix(name, ".idx") {
		return false, fmt.Errorf("invalid reference index name %q", name)
	}
	sharedMu.Lock()
	defer sharedMu.Unlock()
	var services []*Service
	for _, service := range sharedServices {
		if filepath.Clean(service.cacheDir) == filepath.Clean(dir) {
			service.mu.Lock()
			services = append(services, service)
		}
	}
	defer func() {
		for _, service := range services {
			service.mu.Unlock()
		}
	}()
	for _, service := range services {
		for _, entry := range service.indexes {
			if entry.status.State == IndexBuilding {
				return false, fmt.Errorf("reference indexes are building")
			}
		}
	}
	path := filepath.Join(dir, name)
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return false, fmt.Errorf("not a regular index file: %s", name)
	} else if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	for _, service := range services {
		key := strings.TrimSuffix(name, ".idx")
		for id, entry := range service.indexes {
			if entry.index != nil && (entry.index.CacheKey == key || entry.index.Source == key) {
				delete(service.indexes, id)
			}
		}
	}
	return err == nil, nil
}
