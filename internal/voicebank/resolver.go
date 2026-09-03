package voicebank

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"utautts/internal/connection"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

type Selection struct {
	Position                  int
	Mora                      frontend.Mora
	Alias                     string
	Kind                      AliasKind
	Composite                 bool
	Transition                *Selection
	FallbackTier              int
	Entry                     oto.Entry
	Candidates                []string
	CandidateCount            int
	TargetScore               float64
	PreferenceScore           float64
	TransitionScore           float64
	JoinScore                 float64
	JoinProbability           float64
	TransitionJoinScore       float64
	TransitionJoinProbability float64
	PathScore                 float64
	SubbankID                 string
	Color                     string
	RequestedTone             string
	ResolvedTone              string
	EntryStatus               string
	EntryValidation           []string
	CandidateRejections       []CandidateRejection
	AcousticTargetScore       float64
	AcousticJoinScore         float64
	SelectionMargin           float64
}

type CandidateRejection struct {
	Alias  string
	Source string
	Reason string
}

const maxCandidatesPerPosition = 32

type SelectionMode string

const (
	SelectionViterbi    SelectionMode = "viterbi"
	SelectionGreedy     SelectionMode = "greedy"
	SelectionTargetOnly SelectionMode = "target-only"
)

type ResolveConfig struct {
	Tone         string
	Color        string
	Mode         SelectionMode
	AliasPolicy  AliasPolicy
	AcousticMode string
	JoinModel    *connection.LearnedModel
}

type MissingAliasError struct {
	Position            int
	Mora                string
	Candidates          []string
	CandidateRejections []CandidateRejection
}

func (e *MissingAliasError) Error() string {
	message := fmt.Sprintf("no voicebank entry for mora %q at position %d (tried: %s)", e.Mora, e.Position, strings.Join(e.Candidates, ", "))
	if len(e.CandidateRejections) > 0 {
		message += fmt.Sprintf("; rejected %d unusable candidate(s)", len(e.CandidateRejections))
	}
	return message
}

func (b *Bank) Resolve(morae []frontend.Mora) ([]Selection, error) {
	return b.ResolveWithConfig(morae, ResolveConfig{})
}

func (b *Bank) ResolveAtTone(morae []frontend.Mora, tone string) ([]Selection, error) {
	return b.ResolveWithConfig(morae, ResolveConfig{Tone: tone})
}

func (b *Bank) ResolveWithConfig(morae []frontend.Mora, cfg ResolveConfig) ([]Selection, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = SelectionViterbi
	}
	if mode != SelectionViterbi && mode != SelectionGreedy && mode != SelectionTargetOnly {
		return nil, fmt.Errorf("unknown selection mode %q", mode)
	}
	policy := cfg.AliasPolicy
	if policy == "" {
		policy = AliasPolicyAuto
	}
	if !policy.valid() {
		return nil, fmt.Errorf("unknown alias policy %q", policy)
	}
	if !validAcousticMode(cfg.AcousticMode) {
		return nil, fmt.Errorf("unknown acoustic selection mode %q", cfg.AcousticMode)
	}
	layers, err := b.candidateLayersWithPolicyMode(morae, cfg.Tone, cfg.Color, policy, cfg.AcousticMode)
	if err != nil {
		return nil, err
	}
	if b.extractor == nil {
		b.extractor = connection.NewExtractor()
	}
	return selectBestPathsWithAcoustic(layers, mode, cfg.JoinModel, b.extractor, cfg.AcousticMode), nil
}

func (b *Bank) candidateLayers(morae []frontend.Mora, tone string) ([][]Selection, error) {
	return b.candidateLayersWithPolicy(morae, tone, "", AliasPolicyAuto)
}

func (b *Bank) candidateLayersWithPolicy(morae []frontend.Mora, tone, color string, policy AliasPolicy) ([][]Selection, error) {
	return b.candidateLayersWithPolicyMode(morae, tone, color, policy, "")
}

func (b *Bank) candidateLayersWithPolicyMode(morae []frontend.Mora, tone, color string, policy AliasPolicy, acousticMode string) ([][]Selection, error) {
	if !validAcousticMode(acousticMode) {
		return nil, fmt.Errorf("unknown acoustic selection mode %q", acousticMode)
	}
	layers := make([][]Selection, 0, len(morae))
	affix, subbank, hasAffix := b.AffixForToneAndColor(tone, color)
	requestedTone := strings.ToUpper(strings.TrimSpace(tone))
	if requestedTone == "" {
		requestedTone = "C4"
	}
	resolvedTone := requestedTone
	if subbank.Tone != "" {
		resolvedTone = subbank.Tone
	}
	if strings.TrimSpace(color) != "" && len(b.Subbanks) > 0 && !hasAffix {
		return nil, fmt.Errorf("voicebank color %q has no subbank for tone %q", color, tone)
	}
	previousVowel := ""
	phraseStart := true
	var previousLayer []Selection
	for position, mora := range morae {
		if mora.Pause {
			layers = append(layers, nil)
			previousVowel = ""
			phraseStart = true
			previousLayer = nil
			continue
		}

		candidateSpecs := aliasCandidatesWithPolicy(mora.Text, previousVowel, phraseStart, policy)
		consonant := mora.Consonant
		if consonant == "" {
			consonant = frontend.ConsonantOf(mora.Text)
		}
		transitionSpecs := vcAliasCandidates(previousVowel, consonant, policy)
		explicitCandidates := mora.Aliases != nil && len(mora.Aliases.Main) > 0
		if explicitCandidates {
			candidateSpecs = explicitMainAliasCandidates(mora.Aliases.Main, mora.Text)
			transitionSpecs = explicitAliasCandidates(mora.Aliases.Transition, AliasVC)
		}
		if hasAffix {
			strictSubbank := subbank.ID != "" && subbank.ID != "prefix.map"
			if strictSubbank {
				affixedCandidates := affixCandidatesWithFallback(candidateSpecs, affix, false)
				affixedTransitions := affixCandidatesWithFallback(transitionSpecs, affix, false)
				if hasUsableCandidateEntries(b, affixedCandidates) {
					candidateSpecs = affixedCandidates
				} else {
					// 専用oto配下に接辞なしaliasを置くOpenUtau音源へフォールバックする。
					candidateSpecs = affixCandidatesWithFallback(candidateSpecs, affix, true)
				}
				if hasUsableCandidateEntries(b, affixedTransitions) {
					transitionSpecs = affixedTransitions
				} else {
					transitionSpecs = affixCandidatesWithFallback(transitionSpecs, affix, true)
				}
			} else {
				candidateSpecs = affixCandidatesWithFallback(candidateSpecs, affix, true)
				transitionSpecs = affixCandidatesWithFallback(transitionSpecs, affix, true)
			}
		}
		if !explicitCandidates {
			candidateSpecs = preferOriginalKanaCandidates(b, candidateSpecs)
		}
		allSpecs := append(append([]aliasCandidate{}, candidateSpecs...), transitionSpecs...)
		candidates := candidateNames(allSpecs)
		var candidatesAtPosition []Selection
		var rejections []CandidateRejection
		type validatedEntry struct {
			entry      oto.Entry
			validation EntryValidation
		}
		validatedEntries := func(alias string, entries []oto.Entry) []validatedEntry {
			valid := make([]validatedEntry, 0, len(entries))
			for _, entry := range entries {
				validation := b.validateEntry(entry)
				if validation.Status == "unusable" {
					rejections = append(rejections, CandidateRejection{Alias: alias, Source: entry.Filename, Reason: validation.Reason})
					continue
				}
				valid = append(valid, validatedEntry{entry: entry, validation: validation})
			}
			return valid
		}
		for _, candidate := range candidateSpecs {
			entries := validatedEntries(candidate.name, b.Entries[candidate.name])
			for _, validated := range entries {
				entry, validation := validated.entry, validated.validation
				main := Selection{
					Position: position, Mora: mora, Alias: candidate.name, Kind: candidate.kind,
					FallbackTier: candidate.tier, Entry: entry, Candidates: candidates,
					TargetScore: candidateScore(candidate.tier, entry),
					SubbankID:   subbank.ID, Color: subbank.Color, RequestedTone: requestedTone,
					ResolvedTone: resolvedTone, EntryStatus: validation.Status, EntryValidation: validation.Checks,
				}
				if !explicitCandidates {
					candidatesAtPosition = append(candidatesAtPosition, main)
				}
				if candidate.kind != AliasCV || isWildcardAlias(candidate.name) || len(transitionSpecs) == 0 {
					if explicitCandidates {
						candidatesAtPosition = append(candidatesAtPosition, main)
					}
					continue
				}
				compositeAdded := false
				for _, transitionSpec := range transitionSpecs {
					for _, validatedTransition := range validatedEntries(transitionSpec.name, b.Entries[transitionSpec.name]) {
						transitionEntry, transitionValidation := validatedTransition.entry, validatedTransition.validation
						transition := Selection{
							Position: position, Mora: mora, Alias: transitionSpec.name, Kind: AliasVC,
							FallbackTier: transitionSpec.tier, Entry: transitionEntry, Candidates: candidates,
							TargetScore: candidateScore(transitionSpec.tier, transitionEntry),
							SubbankID:   subbank.ID, Color: subbank.Color, RequestedTone: requestedTone,
							ResolvedTone: resolvedTone, EntryStatus: transitionValidation.Status,
							EntryValidation: transitionValidation.Checks,
						}
						composite := main
						composite.Composite = true
						composite.Transition = &transition
						composite.TransitionScore = transition.TargetScore
						candidatesAtPosition = append(candidatesAtPosition, composite)
						compositeAdded = true
					}
				}
				if explicitCandidates && !compositeAdded {
					candidatesAtPosition = append(candidatesAtPosition, main)
				}
			}
		}
		for index := range candidatesAtPosition {
			candidatesAtPosition[index].CandidateRejections = append([]CandidateRejection(nil), rejections...)
			if candidatesAtPosition[index].Transition != nil {
				candidatesAtPosition[index].Transition.CandidateRejections = append([]CandidateRejection(nil), rejections...)
			}
		}
		if len(candidatesAtPosition) == 0 {
			if mora.Vowel == "cl" {
				candidatesAtPosition = []Selection{{
					Position: position, Mora: mora, Alias: "<closure>",
					Kind: AliasOther, FallbackTier: 0,
					Candidates: candidates, CandidateCount: 1,
					TargetScore: 100,
				}}
				layers = append(layers, candidatesAtPosition)
				previousLayer = candidatesAtPosition
				previousVowel = mora.Vowel
				phraseStart = false
				continue
			}
			return nil, &MissingAliasError{Position: position, Mora: mora.Text, Candidates: candidates, CandidateRejections: rejections}
		}
		applyCompositePreferences(candidatesAtPosition, policy)
		b.populateAcousticScores(candidatesAtPosition, previousLayer, acousticMode)
		if len(candidatesAtPosition) > maxCandidatesPerPosition {
			sort.SliceStable(candidatesAtPosition, func(i, j int) bool {
				left := localCandidateScore(candidatesAtPosition[i], acousticMode)
				right := localCandidateScore(candidatesAtPosition[j], acousticMode)
				return left > right
			})
			candidatesAtPosition = candidatesAtPosition[:maxCandidatesPerPosition]
		}
		for index := range candidatesAtPosition {
			candidatesAtPosition[index].CandidateCount = len(candidatesAtPosition)
			if candidatesAtPosition[index].Transition != nil {
				candidatesAtPosition[index].Transition.CandidateCount = len(candidatesAtPosition)
			}
		}
		layers = append(layers, candidatesAtPosition)
		previousLayer = candidatesAtPosition
		previousVowel = mora.Vowel
		phraseStart = false
	}
	return layers, nil
}

// candidateScoreはalias優先度とoto.iniの整合性から重複候補を選ぶ。
func candidateScore(candidateTier int, entry oto.Entry) float64 {
	score := 100 - float64(candidateTier)*10
	if entry.Preutterance >= 0 {
		score += 4
	} else {
		score -= 30 + math.Abs(entry.Preutterance)
	}
	if entry.Fixed >= entry.Preutterance && entry.Fixed >= 0 {
		score += 4
	} else {
		score -= 20 + math.Abs(entry.Preutterance-entry.Fixed)
	}
	if entry.Overlap <= entry.Preutterance {
		score += 4
	} else {
		score -= 20 + math.Abs(entry.Overlap-entry.Preutterance)
	}
	if entry.Offset >= 0 {
		score += 2
	} else {
		score -= 20
	}
	return score
}

func localCandidateScore(candidate Selection, acousticMode string) float64 {
	score := candidate.TargetScore + candidate.PreferenceScore
	if acousticMode == AcousticModeApply {
		score += candidate.AcousticTargetScore
	}
	return score
}

func hasUsableCandidateEntries(bank *Bank, candidates []aliasCandidate) bool {
	for _, candidate := range candidates {
		for _, entry := range bank.Entries[candidate.name] {
			if bank.validateEntry(entry).Status != "unusable" {
				return true
			}
		}
	}
	return false
}

type aliasCandidate struct {
	name       string
	tier       int
	kind       AliasKind
	equivalent bool
}

func explicitAliasCandidates(names []string, kind AliasKind) []aliasCandidate {
	result := make([]aliasCandidate, 0, len(names))
	for tier, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			result = append(result, aliasCandidate{name: name, tier: tier, kind: kind})
		}
	}
	return uniqueCandidates(result)
}

func explicitMainAliasCandidates(names []string, fallback string) []aliasCandidate {
	result := explicitAliasCandidates(names, AliasOther)
	for index := range result {
		if result[index].name == fallback {
			result[index].kind = AliasCV
		}
	}
	return result
}

func preferOriginalKanaCandidates(bank *Bank, candidates []aliasCandidate) []aliasCandidate {
	originals := make([]aliasCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.equivalent {
			originals = append(originals, candidate)
		}
	}
	if !hasUsableCandidateEntries(bank, originals) {
		return candidates
	}

	result := make([]aliasCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.equivalent {
			result = append(result, candidate)
		}
	}
	return result
}

func affixCandidates(base []aliasCandidate, affix Affix) []aliasCandidate {
	return affixCandidatesWithFallback(base, affix, true)
}

func affixCandidatesWithFallback(base []aliasCandidate, affix Affix, allowUnprefixed bool) []aliasCandidate {
	result := make([]aliasCandidate, 0, len(base)*2)
	for _, candidate := range base {
		result = append(result, aliasCandidate{name: affix.Prefix + candidate.name + affix.Suffix, tier: candidate.tier, kind: candidate.kind, equivalent: candidate.equivalent})
		if allowUnprefixed {
			result = append(result, aliasCandidate{name: candidate.name, tier: candidate.tier + 1, kind: candidate.kind, equivalent: candidate.equivalent})
		}
	}
	return uniqueCandidates(result)
}

func aliasCandidates(mora, previousVowel string, phraseStart bool) []aliasCandidate {
	return aliasCandidatesWithPolicy(mora, previousVowel, phraseStart, AliasPolicyAuto)
}

// equivalentKanaForms returns kana that are pronounced identically to the
// given mora in modern standard Japanese. These are safe fallbacks for
// voicebanks that lack a dedicated recording:
//
//	を = お   (the particle を is pronounced "o")
//	ぢ = じ   (di and ji merged)
//	づ = ず   (du and zu merged)
//	ゐ = い   (archaic wi = i)
//	ゑ = え   (archaic we = e)
//
// Small-kana combinations (てぃ, とぅ, ふぁ, ...) are NOT included because
// they are genuinely different sounds.
func equivalentKanaForms(mora string) []string {
	switch mora {
	case "を":
		return []string{"お"}
	case "ぢ":
		return []string{"じ"}
	case "づ":
		return []string{"ず"}
	case "ゐ":
		return []string{"い"}
	case "ゑ":
		return []string{"え"}
	}
	return nil
}

// aliasForm is one surface form offered for a mora. fallback is an extra tier
// penalty applied to phonetically-equivalent alternates (を→お etc.) so a
// bank that owns both recordings always prefers the original kana.
type aliasForm struct {
	text       string
	fallback   int
	equivalent bool
}

func aliasCandidatesWithPolicy(mora, previousVowel string, phraseStart bool, policy AliasPolicy) []aliasCandidate {
	forms := make([]aliasForm, 0, 4)
	if mora == "ー" {
		if vowelKana := map[string]string{"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お"}[previousVowel]; vowelKana != "" {
			forms = append(forms, aliasForm{text: vowelKana}, aliasForm{text: toKatakana(vowelKana)})
		}
	}
	// The mora itself plus phonetically identical alternates (modern
	// standard Japanese), so voicebanks that lack a dedicated recording
	// still synthesize the mora: を=お, ぢ=じ, づ=ず, ゐ=い, ゑ=え.
	// Alternates carry a +1 tier penalty: the original kana must win even
	// when both recordings exist, independent of oto.ini entry quality.
	base := []aliasForm{{text: mora}}
	for _, equivalent := range equivalentKanaForms(mora) {
		base = append(base, aliasForm{text: equivalent, fallback: 1, equivalent: true})
	}
	for _, form := range base {
		forms = append(forms, form)
		if katakana := toKatakana(form.text); katakana != form.text {
			forms = append(forms, aliasForm{text: katakana, fallback: form.fallback, equivalent: form.equivalent})
		}
	}

	var candidates []aliasCandidate
	allowVCVTarget := mora != "っ"
	if policy != AliasPolicyCVOnly && allowVCVTarget && phraseStart {
		for _, form := range forms {
			candidates = append(candidates, aliasCandidate{name: "- " + form.text, tier: form.fallback, kind: AliasVCV, equivalent: form.equivalent})
		}
	} else if policy != AliasPolicyCVOnly && allowVCVTarget && previousVowel != "" && previousVowel != "cl" {
		for _, form := range forms {
			candidates = append(candidates, aliasCandidate{name: previousVowel + " " + form.text, tier: form.fallback, kind: AliasVCV, equivalent: form.equivalent})
		}
	}
	for _, form := range forms {
		candidates = append(candidates, aliasCandidate{name: form.text, tier: policyTier(policy, 1, AliasCV) + form.fallback, kind: AliasCV, equivalent: form.equivalent})
	}
	if policy != AliasPolicyCVOnly && !phraseStart {
		for _, form := range forms {
			candidates = append(candidates, aliasCandidate{name: "* " + form.text, tier: policyTier(policy, 2, AliasCV) + form.fallback, kind: AliasCV, equivalent: form.equivalent})
		}
	}
	return uniqueCandidates(candidates)
}

func vcAliasCandidates(previousVowel, consonant string, policy AliasPolicy) []aliasCandidate {
	if policy == AliasPolicyCVOnly || previousVowel == "" || previousVowel == "cl" || consonant == "" || consonant == "cl" {
		return nil
	}
	contexts := vowelContextForms(previousVowel)
	result := make([]aliasCandidate, 0, len(contexts))
	for _, context := range contexts {
		result = append(result, aliasCandidate{name: context + " " + consonant, tier: vcPolicyTier(policy), kind: AliasVC})
	}
	return uniqueCandidates(result)
}

func vowelContextForms(vowel string) []string {
	forms := []string{vowel}
	if kana := map[string]string{"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お", "n": "ん"}[vowel]; kana != "" {
		forms = append(forms, kana)
	}
	return forms
}

func vcPolicyTier(policy AliasPolicy) int {
	if policy == AliasPolicyVCVPrefer {
		return 2
	}
	return 0
}

func applyCompositePreferences(candidates []Selection, policy AliasPolicy) {
	hasComposite := false
	for _, candidate := range candidates {
		if candidate.Composite {
			hasComposite = true
			break
		}
	}
	if !hasComposite {
		return
	}
	for index := range candidates {
		candidate := &candidates[index]
		if candidate.Composite {
			candidate.PreferenceScore = compositePreferenceScore(policy)
			continue
		}
		if candidate.Kind == AliasVCV && policy != AliasPolicyCVVCPrefer {
			candidate.PreferenceScore = 10
		}
	}
}

func compositePreferenceScore(policy AliasPolicy) float64 {
	switch policy {
	case AliasPolicyVCVPrefer:
		return 22
	case AliasPolicyCVVCPrefer:
		return 12
	default:
		return 12
	}
}

func policyTier(policy AliasPolicy, tier int, kind AliasKind) int {
	if policy == AliasPolicyVCVPrefer && kind != AliasVCV {
		return tier + 2
	}
	if policy == AliasPolicyCVVCPrefer && kind == AliasVCV {
		return tier + 2
	}
	return tier
}

func toKatakana(value string) string {
	var result strings.Builder
	for _, r := range value {
		if r >= 'ぁ' && r <= 'ゖ' {
			r += 0x60
		}
		result.WriteRune(r)
	}
	return result.String()
}

func uniqueCandidates(values []aliasCandidate) []aliasCandidate {
	indices := map[string]int{}
	result := make([]aliasCandidate, 0, len(values))
	for _, value := range values {
		if index, ok := indices[value.name]; ok {
			result[index].tier = min(result[index].tier, value.tier)
			result[index].equivalent = result[index].equivalent && value.equivalent
			if result[index].kind == AliasOther {
				result[index].kind = value.kind
			}
		} else {
			indices[value.name] = len(result)
			result = append(result, value)
		}
	}
	return result
}

func candidateNames(candidates []aliasCandidate) []string {
	result := make([]string, len(candidates))
	for index, candidate := range candidates {
		result[index] = candidate.name
	}
	return result
}

func isWildcardAlias(alias string) bool {
	parts := strings.Fields(alias)
	return len(parts) >= 2 && strings.Contains(parts[0], "*")
}
