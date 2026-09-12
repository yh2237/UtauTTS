package voicebank

import (
	"strings"
	"unicode"
)

type AliasKind string

const (
	AliasCV    AliasKind = "CV"
	AliasVCV   AliasKind = "VCV"
	AliasVC    AliasKind = "VC"
	AliasOther AliasKind = "other"
)

type AliasPolicy string

const (
	AliasPolicyAuto       AliasPolicy = "auto"
	AliasPolicyEnhanced   AliasPolicy = "cvvc-enhanced"
	AliasPolicyVCVPrefer  AliasPolicy = "vcv-prefer"
	AliasPolicyCVVCPrefer AliasPolicy = "cvvc-prefer"
	AliasPolicyCVOnly     AliasPolicy = "cv-only"
)

func (p AliasPolicy) valid() bool {
	return p == "" || p == AliasPolicyAuto || p == AliasPolicyEnhanced || p == AliasPolicyVCVPrefer || p == AliasPolicyCVVCPrefer || p == AliasPolicyCVOnly
}

type AliasCapabilities struct {
	Counts         map[AliasKind]int
	VCVContexts    map[string]int
	VCContexts     map[string]int
	InitialAliases int
	ContextVCV     int
	HasVCV         bool
	HasVC          bool
	HasInitialVCV  bool
	HasNContextVCV bool
}

func ClassifyAlias(alias string) AliasKind {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return AliasOther
	}
	parts := strings.Fields(alias)
	if len(parts) == 1 {
		if containsKana(parts[0]) {
			return AliasCV
		}
		return AliasOther
	}
	if len(parts) != 2 {
		return AliasOther
	}
	if parts[0] == "*" && containsKana(parts[1]) {
		return AliasCV
	}
	if (parts[0] == "-" || isVowelContext(parts[0])) && containsKana(parts[1]) {
		return AliasVCV
	}
	if (containsKana(parts[0]) || isVowelContext(parts[0])) && isConsonantContext(parts[1]) {
		return AliasVC
	}
	return AliasOther
}

func containsKana(value string) bool {
	for _, r := range value {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
			return true
		}
	}
	return false
}

// IsInitialContextAlias は日本語VCVの語頭形式を判定する
// 先行母音を持たないため実VCV数には含めない
func IsInitialContextAlias(alias string) bool {
	parts := strings.Fields(strings.TrimSpace(alias))
	return len(parts) >= 2 && parts[0] == "-" && containsKana(parts[1])
}

// IsContextVCVAlias は先行母音を持つVCVだけを判定する
func IsContextVCVAlias(alias string) bool {
	parts := strings.Fields(strings.TrimSpace(alias))
	return len(parts) >= 2 && isVowelContext(parts[0]) && containsKana(parts[1])
}

// IsSingleCVSelection は選択された主ユニットが単独音かを判定する
// 語頭の「- CV」は先行母音を持たないため単独音として扱う
func IsSingleCVSelection(selection Selection) bool {
	kind := selection.Kind
	if kind == "" {
		kind = ClassifyAlias(selection.Alias)
	}
	if selection.Composite || selection.Transition != nil || len(selection.Endings) > 0 {
		return false
	}
	if kind == AliasCV {
		return true
	}
	return (kind == AliasVCV || kind == AliasOther) && IsInitialContextAlias(selection.Alias)
}

// IsSingleCVSelections は主ユニット全体を単独音として扱えるか判定する
func IsSingleCVSelections(selections []Selection) bool {
	seen := false
	for _, selection := range selections {
		if selection.Mora.Pause {
			continue
		}
		seen = true
		if !IsSingleCVSelection(selection) {
			return false
		}
	}
	return seen
}

func isVowelContext(value string) bool {
	value = strings.ToLower(value)
	switch value {
	case "a", "i", "u", "e", "o", "n", "あ", "い", "う", "え", "お", "ん", "ア", "イ", "ウ", "エ", "オ", "ン":
		return true
	default:
		return false
	}
}

func isConsonantContext(value string) bool {
	if len(value) == 0 || len(value) > 4 {
		return false
	}
	for _, r := range value {
		isLetter := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
		if !isLetter || strings.ContainsRune("aeiouAEIOU", r) {
			return false
		}
	}
	return true
}

func (b *Bank) AliasCounts() map[AliasKind]int {
	return b.AliasCapabilities().Counts
}

// RecommendCVVCEnhancedは、少数の補助VCを持つVCV音源を除外してCVVC向け音源を判定する。
func (b *Bank) RecommendCVVCEnhanced() bool {
	capabilities := b.AliasCapabilities()
	vc := capabilities.Counts[AliasVC]
	vcv := capabilities.ContextVCV
	return vc >= 24 && (vcv == 0 || vc*2 >= vcv)
}

func (b *Bank) AliasCapabilities() AliasCapabilities {
	capabilities := AliasCapabilities{
		Counts:      map[AliasKind]int{},
		VCVContexts: map[string]int{},
		VCContexts:  map[string]int{},
	}
	for alias := range b.Entries {
		kind := ClassifyAlias(alias)
		if kind == AliasOther && (IsInitialContextAlias(alias) || IsContextVCVAlias(alias)) {
			kind = AliasVCV
		}
		capabilities.Counts[kind]++
		parts := strings.Fields(alias)
		if len(parts) != 2 {
			if IsInitialContextAlias(alias) || IsContextVCVAlias(alias) {
				parts = parts[:2]
			} else {
				continue
			}
		}
		switch {
		case IsInitialContextAlias(alias):
			capabilities.HasVCV = true
			capabilities.VCVContexts[parts[0]]++
			capabilities.InitialAliases++
			capabilities.HasInitialVCV = true
			if parts[0] == "n" {
				capabilities.HasNContextVCV = true
			}
		case IsContextVCVAlias(alias):
			capabilities.HasVCV = true
			capabilities.VCVContexts[parts[0]]++
			capabilities.ContextVCV++
			if parts[0] == "n" {
				capabilities.HasNContextVCV = true
			}
		case kind == AliasVC:
			capabilities.HasVC = true
			capabilities.VCContexts[parts[0]]++
		}
	}
	return capabilities
}
