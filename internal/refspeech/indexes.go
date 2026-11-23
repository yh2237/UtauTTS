package refspeech

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/yh2237/UtauIDX/index"
	"github.com/yh2237/UtauIDX/speechpool"
)

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
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".utsp") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		header, err := index.ReadHeader(path)
		if err != nil {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		name := ""
		if header.Source != "" {
			name = filepath.Base(filepath.Clean(header.Source))
		}
		result = append(result, IndexInfo{
			Name: entry.Name(), Path: path, Voicebank: name, Model: strings.Join(indexChannelIDs(header), ","),
			Version: header.Version, SizeBytes: info.Size(), ModifiedMS: info.ModTime().UnixMilli(),
		})
	}
	return result, nil
}

func indexChannelIDs(header index.Header) []string {
	ids := make([]string, len(header.Channels))
	for i, channel := range header.Channels {
		ids[i] = channel.ID
	}
	slices.Sort(ids)
	return ids
}

func matchesSource(idx *index.Index, root, key string) bool {
	return idx.Source == filepath.Clean(root) && idx.CacheKey == key
}

// PrepareStatusは索引の準備を開始し、完成を待たずに状態を返す。
func (s *Service) PrepareStatus(root string, color string, entries []speechpool.Entry, world string) IndexStatus {
	s.dropStale(root, color, entries)
	entry := s.entry(root, color, entries, world)
	s.mu.Lock()
	defer s.mu.Unlock()
	return entry.status
}

// DeleteIndexはキャッシュ内の索引を削除し、読み込み済みの索引も捨てる。
func DeleteIndex(dir, name string) (bool, error) {
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\:`) || !strings.HasSuffix(name, ".utsp") {
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
		key := strings.TrimSuffix(name, ".utsp")
		for id, entry := range service.indexes {
			if entry.index != nil && entry.index.CacheKey == key {
				delete(service.indexes, id)
			}
		}
	}
	return err == nil, nil
}
