package voicebank

import (
	"sort"
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/oto"
)

// 先読み用の候補を返す。WAVの検証と原音選択は合成時に行う。
func (b *Bank) CandidateSourcePaths(morae []frontend.Mora, cfg ResolveConfig) []string {
	policy := cfg.AliasPolicy
	if policy == "" {
		policy = AliasPolicyAuto
	}
	affix, _, hasAffix := b.AffixForToneAndColor(cfg.Tone, cfg.Color)
	paths := make(map[string]bool)
	collect := func(specs []aliasCandidate) {
		if hasAffix {
			specs = affixCandidatesWithFallback(specs, affix, true)
		}
		for _, spec := range specs {
			for _, entry := range b.Entries[spec.name] {
				if entry.Filename != "" {
					paths[entry.Filename] = true
				}
			}
		}
	}
	previous, initial := "", true
	for _, mora := range morae {
		if mora.Pause {
			previous, initial = "", true
			continue
		}
		if mora.Aliases != nil && len(mora.Aliases.Main) > 0 {
			collect(explicitMainAliasCandidates(mora.Aliases.Main, mora.Aliases.MainKinds, mora.Text))
			collect(explicitAliasCandidates(mora.Aliases.Transition, AliasVC))
		} else {
			collect(aliasCandidatesWithPolicy(mora.Text, previous, initial, policy))
			if previous == "cl" {
				collect(aliasCandidatesWithPolicy(mora.Text, "", true, policy))
			}
			consonant := mora.Consonant
			if consonant == "" {
				consonant = frontend.ConsonantOf(mora.Text)
			}
			collect(vcAliasCandidates(previous, consonant, policy))
		}
		if mora.Aliases != nil {
			for _, aliases := range mora.Aliases.Endings {
				collect(explicitAliasCandidates(aliases, AliasOther))
			}
			for _, alternatives := range mora.Aliases.EndingFallbacks {
				for _, alternative := range alternatives {
					collect(explicitAliasCandidates(alternative.Aliases, AliasOther))
				}
			}
		}
		previous, initial = mora.Vowel, false
	}
	result := make([]string, 0, len(paths))
	for path := range paths {
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

// ReferenceRecordingは参照照合の索引に使う録音と、その録音を使う原音。
type ReferenceRecording struct {
	Path    string
	Entries []oto.Entry
}

// ReferenceRecordingsは参照照合の索引に使う録音を返す。音高や声色ごとの収録があれば、既定の音高のcolorの声だけを使う。
// colorがない音源では通常の声を使う。録音の範囲を決めるため、同じ録音を使う原音はすべて返す。
func (b *Bank) ReferenceRecordings(color string) []ReferenceRecording {
	affix, _, hasAffix := b.AffixForToneAndColor("", color)
	if !hasAffix && color != "" {
		affix, _, hasAffix = b.AffixForToneAndColor("", "")
	}
	// 接頭辞も接尾辞も空の通常の声は、どの別名にも当てはまる。他の声色や音高の別名は除く。
	var others []Affix
	for _, subbank := range b.Subbanks {
		others = append(others, Affix{Prefix: subbank.Prefix, Suffix: subbank.Suffix})
	}
	for _, other := range b.PrefixMap {
		others = append(others, other)
	}
	matches := func(alias string, affix Affix) bool {
		return strings.HasPrefix(alias, affix.Prefix) && strings.HasSuffix(alias, affix.Suffix)
	}
	belongsElsewhere := func(alias string) bool {
		for _, other := range others {
			if len(other.Prefix)+len(other.Suffix) > len(affix.Prefix)+len(affix.Suffix) && matches(alias, other) {
				return true
			}
		}
		return false
	}
	collect := func(affixed bool) map[string][]oto.Entry {
		paths := make(map[string][]oto.Entry)
		for alias, entries := range b.Entries {
			if affixed && (!matches(alias, affix) || belongsElsewhere(alias)) {
				continue
			}
			for _, entry := range entries {
				if entry.Filename != "" {
					paths[entry.Filename] = nil
				}
			}
		}
		return paths
	}
	paths := collect(hasAffix)
	if len(paths) == 0 {
		// 既定の音高の別名が見つからない音源は、すべての録音を使う。
		paths = collect(false)
	}
	for _, entries := range b.Entries {
		for _, entry := range entries {
			if list, ok := paths[entry.Filename]; ok {
				paths[entry.Filename] = append(list, entry)
			}
		}
	}
	result := make([]ReferenceRecording, 0, len(paths))
	for path, entries := range paths {
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].Offset != entries[j].Offset {
				return entries[i].Offset < entries[j].Offset
			}
			return entries[i].Blank < entries[j].Blank
		})
		result = append(result, ReferenceRecording{Path: path, Entries: entries})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}
