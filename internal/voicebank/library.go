package voicebank

import (
	"errors"
	"os"
	"sort"
	"sync"
)

// Libraryは発見済み音源と安定IDをまとめて保持する。
// 責務は発見と選択のみで、表示用メタデータの導出は呼び出し側に委ねる。
type Library struct {
	root  string
	mu    sync.RWMutex
	items map[string]Summary
}

// LibraryItemは発見済み音源と安定IDの組。
type LibraryItem struct {
	ID      string
	Summary Summary
}

// NewLibraryはconfiguredをルートとする空のライブラリを生成する。
func NewLibrary(configured string) *Library {
	return &Library{root: ResolveDirectory(configured), items: make(map[string]Summary)}
}

// Rootは発見と安定IDに使う解決済みディレクトリを返す。
func (l *Library) Root() string {
	if l == nil {
		return ""
	}
	return l.root
}

// Reloadはライブラリをアトミックに置き換える。ディレクトリの欠落・空は
// アプリ起動時と同様に有効な空ライブラリとして扱う。
func (l *Library) Reload() error {
	if l == nil {
		return errors.New("voicebank library is not configured")
	}
	summaries, err := Discover(l.root)
	if err != nil && !errors.Is(err, ErrNoOto) && !os.IsNotExist(err) {
		l.replace(nil)
		return err
	}
	next := make(map[string]Summary, len(summaries))
	for _, summary := range summaries {
		next[StableID(l.root, summary.Path)] = summary
	}
	l.replace(next)
	return nil
}

// Addはディレクトリ全体を走査せず、検査済みの音源を登録する。
func (l *Library) Add(summary Summary) string {
	if l == nil || summary.Path == "" {
		return ""
	}
	id := StableID(l.root, summary.Path)
	l.mu.Lock()
	if l.items == nil {
		l.items = make(map[string]Summary)
	}
	l.items[id] = summary
	l.mu.Unlock()
	return id
}

// Resolveは明示選択された音源を返す。idが空なら安定順で先頭の項目を返す。
func (l *Library) Resolve(id string) (Summary, bool) {
	if l == nil {
		return Summary{}, false
	}
	l.mu.RLock()
	defer l.mu.RUnlock()
	if id != "" {
		item, ok := l.items[id]
		return item, ok
	}
	keys := make([]string, 0, len(l.items))
	for key := range l.items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return Summary{}, false
	}
	item, ok := l.items[keys[0]]
	return item, ok
}

// ListはID順の安定したスナップショットを返す。
func (l *Library) List() []LibraryItem {
	if l == nil {
		return nil
	}
	l.mu.RLock()
	items := make([]LibraryItem, 0, len(l.items))
	for id, summary := range l.items {
		items = append(items, LibraryItem{ID: id, Summary: summary})
	}
	l.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (l *Library) replace(items map[string]Summary) {
	if items == nil {
		items = make(map[string]Summary)
	}
	l.mu.Lock()
	l.items = items
	l.mu.Unlock()
}
