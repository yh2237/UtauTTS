package voicebank

import (
	"errors"
	"os"
	"sort"
	"sync"
)

// Library keeps the discovered voicebanks and their stable IDs together.
// It intentionally only owns discovery and selection; callers remain free to
// derive presentation metadata appropriate for their own boundary.
type Library struct {
	root  string
	mu    sync.RWMutex
	items map[string]Summary
}

// LibraryItem is a discovered voicebank paired with its stable ID.
type LibraryItem struct {
	ID      string
	Summary Summary
}

// NewLibrary creates an initially empty library rooted at configured.
func NewLibrary(configured string) *Library {
	return &Library{root: ResolveDirectory(configured), items: make(map[string]Summary)}
}

// Root returns the resolved directory used for discovery and stable IDs.
func (l *Library) Root() string {
	if l == nil {
		return ""
	}
	return l.root
}

// Reload replaces the library atomically. A missing or empty directory is a
// valid empty library, matching the application's startup behavior.
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

// Add registers an already-inspected voicebank without a directory-wide scan.
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

// Resolve returns an explicitly selected voicebank, or the stable first item
// when id is empty.
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

// List returns a stable snapshot ordered by ID.
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
