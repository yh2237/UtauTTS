package voicebank

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/connection"
	"github.com/yh2237/UtauTTS/internal/oto"
)

var ErrNoOto = errors.New("voicebank contains no oto.ini")

type Diagnostic struct {
	Path    string
	Line    int
	Message string
}

type Bank struct {
	Root              string
	Name              string
	OtoFiles          []string
	Entries           map[string][]oto.Entry
	PrefixMap         map[string]Affix
	Subbanks          []Subbank
	CharacterYAML     string
	DefaultPhonemizer string
	ARPAsing          map[string]string
	Presamp           *Presamp
	Diagnostics       []Diagnostic
	extractor         *connection.Extractor
	validationMu      sync.Mutex
	validationCache   map[oto.Entry]cachedEntryValidation
	speechProfiles    map[oto.Entry]SpeechProfile
}

func Load(root string) (*Bank, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !strings.EqualFold(filepath.Base(absRoot), "oto.ini") {
			return nil, fmt.Errorf("voicebank path must be a directory or oto.ini: %s", root)
		}
		absRoot = filepath.Dir(absRoot)
	}
	// 展開時に包まれたフォルダを渡されても、音源の一覧と同じ内側のフォルダを使う。
	if !hasDirectOto(absRoot) && !hasVoicebankMetadata(absRoot) {
		if nested, err := findNestedVoicebankRoot(absRoot); err == nil {
			absRoot = nested
		}
	}

	var otoFiles []string
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(entry.Name(), "oto.ini") {
			otoFiles = append(otoFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(otoFiles) == 0 {
		return nil, ErrNoOto
	}
	sort.Strings(otoFiles)

	bank := &Bank{
		Root:      absRoot,
		Name:      filepath.Base(absRoot),
		OtoFiles:  otoFiles,
		Entries:   map[string][]oto.Entry{},
		PrefixMap: map[string]Affix{},
		extractor: connection.NewExtractor(),
	}
	rootEntries, _ := os.ReadDir(absRoot)
	bank.loadMetadata(rootEntries)
	bank.loadARPAsing(rootEntries)
	bank.loadPresamp(rootEntries)
	pathValidator := newSourcePathValidator(absRoot)
	for _, path := range otoFiles {
		ini, err := oto.ReadIni(path)
		if err != nil {
			return nil, err
		}
		for alias, entries := range ini.Entries {
			for _, entry := range entries {
				if !pathValidator.within(entry.Filename) {
					// 文字化けしたパスは、その行だけ無視する。
					if lexicallyWithin(absRoot, entry.Filename) && unusablePath(entry.Filename) {
						bank.Diagnostics = append(bank.Diagnostics, Diagnostic{Path: path, Message: fmt.Sprintf("skipped oto entry %q with an unusable source path", entry.Filename)})
						continue
					}
					return nil, fmt.Errorf("oto entry %q in %s points outside voicebank root", entry.Filename, path)
				}
				entry.SourceGroup = sourceGroupForOto(absRoot, path)
				entriesForAlias := bank.Entries[alias]
				entriesForAlias = append(entriesForAlias, entry)
				bank.Entries[alias] = entriesForAlias
			}
		}
		for _, diagnostic := range ini.Diagnostics {
			bank.Diagnostics = append(bank.Diagnostics, Diagnostic{
				Path:    path,
				Line:    diagnostic.Line,
				Message: diagnostic.Message,
			})
		}
	}
	return bank, nil
}

func sourceGroupForOto(root, otoPath string) string {
	relative, err := filepath.Rel(root, filepath.Dir(otoPath))
	if err != nil || relative == "." || relative == "" {
		return "root"
	}
	return filepath.ToSlash(relative)
}

type resolvedSourceParent struct {
	path string
	err  error
}

type sourcePathValidator struct {
	root         string
	resolvedRoot string
	rootErr      error
	parents      map[string]resolvedSourceParent
	checked      map[string]bool
	// entriesはディレクトリごとの名前と、シンボリックリンクかどうか。読めなかったディレクトリはnil。
	entries map[string]map[string]bool
}

func newSourcePathValidator(root string) *sourcePathValidator {
	root = filepath.Clean(root)
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	return &sourcePathValidator{
		root: root, resolvedRoot: resolvedRoot, rootErr: rootErr,
		parents: make(map[string]resolvedSourceParent), checked: make(map[string]bool),
		entries: make(map[string]map[string]bool),
	}
}

func (v *sourcePathValidator) within(candidate string) bool {
	candidate = filepath.Clean(candidate)
	if allowed, ok := v.checked[candidate]; ok {
		return allowed
	}
	allowed := v.check(candidate)
	v.checked[candidate] = allowed
	return allowed
}

func lexicallyWithin(root, candidate string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

// ファイルの欠落と、不正なパスを区別する。
func unusablePath(candidate string) bool {
	_, err := os.Lstat(candidate)
	return err != nil && !os.IsNotExist(err) && !os.IsPermission(err)
}

func (v *sourcePathValidator) check(candidate string) bool {
	if !lexicallyWithin(v.root, candidate) {
		return false
	}

	if v.rootErr != nil {
		return true
	}
	parent := filepath.Dir(candidate)
	resolved, ok := v.parents[parent]
	if !ok {
		resolved.path, resolved.err = filepath.EvalSymlinks(parent)
		v.parents[parent] = resolved
	}
	if resolved.err != nil {
		return true
	}
	resolvedCandidate := filepath.Join(resolved.path, filepath.Base(candidate))
	if symlink, found := v.entry(parent, filepath.Base(candidate)); found {
		if symlink {
			var err error
			if resolvedCandidate, err = filepath.EvalSymlinks(candidate); err != nil {
				return false
			}
		}
	} else if info, err := os.Lstat(candidate); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			resolvedCandidate, err = filepath.EvalSymlinks(candidate)
			if err != nil {
				return false
			}
		}
	} else if !os.IsNotExist(err) {
		return false
	}
	resolvedRelative, err := filepath.Rel(v.resolvedRoot, resolvedCandidate)
	return err == nil && resolvedRelative != ".." && !strings.HasPrefix(resolvedRelative, ".."+string(filepath.Separator)) && !filepath.IsAbs(resolvedRelative)
}

// entryは、ディレクトリの一覧に名前があるかと、シンボリックリンクかを返す。
// 録音ごとにLstatを呼ばないよう、一覧はディレクトリごとに1回だけ読む。
func (v *sourcePathValidator) entry(dir, name string) (symlink, found bool) {
	names, ok := v.entries[dir]
	if !ok {
		if list, err := os.ReadDir(dir); err == nil {
			names = make(map[string]bool, len(list))
			for _, item := range list {
				names[item.Name()] = item.Type()&os.ModeSymlink != 0
			}
		}
		v.entries[dir] = names
	}
	symlink, found = names[name]
	return symlink, found
}

func (b *Bank) Aliases() []string {
	aliases := make([]string, 0, len(b.Entries))
	for alias := range b.Entries {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return aliases
}

func (b *Bank) EntryCount() int {
	count := 0
	for _, entries := range b.Entries {
		count += len(entries)
	}
	return count
}
