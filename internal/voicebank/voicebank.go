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

	"utautts/internal/connection"
	"utautts/internal/oto"
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
	bank.loadMetadata()
	bank.loadARPAsing()
	bank.loadPresamp()
	pathValidator := newSourcePathValidator(absRoot)
	for _, path := range otoFiles {
		ini, err := oto.ReadIni(path)
		if err != nil {
			return nil, err
		}
		for alias, entries := range ini.Entries {
			for _, entry := range entries {
				if !pathValidator.within(entry.Filename) {
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

func sourcePathWithin(root, candidate string) bool {
	return newSourcePathValidator(root).within(candidate)
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
}

func newSourcePathValidator(root string) *sourcePathValidator {
	root = filepath.Clean(root)
	resolvedRoot, rootErr := filepath.EvalSymlinks(root)
	return &sourcePathValidator{
		root: root, resolvedRoot: resolvedRoot, rootErr: rootErr,
		parents: make(map[string]resolvedSourceParent), checked: make(map[string]bool),
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

func (v *sourcePathValidator) check(candidate string) bool {
	relative, err := filepath.Rel(v.root, candidate)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
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
	if info, err := os.Lstat(candidate); err == nil {
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
