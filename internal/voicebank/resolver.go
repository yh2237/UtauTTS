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
	Position            int
	Mora                frontend.Mora
	Alias               string
	Kind                AliasKind
	Composite           bool
	Transition          *Selection
	Endings             []Selection
	EndingCandidates    [][]Selection
	EndingAlternatives  []Selection
	EndingIndex         int
	CodaPhones          []string
	CodaStart           int
	MissingPhones       []SpeechGap
	FallbackTier        int
	Entry               oto.Entry
	Candidates          []string
	CandidateCount      int
	TargetScore         float64
	PreferenceScore     float64
	TransitionScore     float64
	JoinScore           float64
	TransitionJoinScore float64
	PathScore           float64
	SubbankID           string
	Color               string
	RequestedTone       string
	ResolvedTone        string
	EntryStatus         string
	EntryValidation     []string
	CandidateRejections []CandidateRejection
}

type CandidateRejection struct {
	Alias  string
	Source string
	Reason string
}

const (
	maxCandidatesPerPosition = 32
	// 同一録音への偏りを避け、別録音の候補を残す。
	minDistinctSourceCandidates = 4
)

type SpeechGap struct {
	Position int      `json:"position"`
	Role     string   `json:"role"`
	Phones   []string `json:"phones"`
	Aliases  []string `json:"aliases"`
}

type ResolveConfig struct {
	Tone        string
	Color       string
	AliasPolicy AliasPolicy
	JoinModel   *connection.JoinModel
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
	if cfg.JoinModel != nil {
		if err := cfg.JoinModel.Validate(); err != nil {
			return nil, fmt.Errorf("join model: %w", err)
		}
	}
	policy := cfg.AliasPolicy
	if policy == "" {
		policy = AliasPolicyAuto
	}
	if !policy.valid() {
		return nil, fmt.Errorf("unknown alias policy %q", policy)
	}
	layers, err := b.candidateLayersWithPolicy(morae, cfg.Tone, cfg.Color, policy)
	if err != nil {
		return nil, err
	}
	if b.extractor == nil {
		b.extractor = connection.NewExtractor()
	}
	extractor := b.extractor
	if cfg.JoinModel != nil {
		extractor = connection.NewExtractorWithModel(cfg.JoinModel)
	}
	return selectBestPaths(layers, extractor), nil
}

func (b *Bank) candidateLayers(morae []frontend.Mora, tone string) ([][]Selection, error) {
	return b.candidateLayersWithPolicy(morae, tone, "", AliasPolicyAuto)
}

func (b *Bank) candidateLayersWithPolicy(morae []frontend.Mora, tone, color string, policy AliasPolicy) ([][]Selection, error) {
	return b.candidateLayersDiagnostic(morae, tone, color, policy, nil)
}

func (b *Bank) candidateLayersDiagnostic(morae []frontend.Mora, tone, color string, policy AliasPolicy, missing *[]MissingAliasError) ([][]Selection, error) {
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
	search := candidateContext{
		bank: b, policy: policy, affix: affix, hasAffix: hasAffix, subbank: subbank,
		requestedTone: requestedTone, resolvedTone: resolvedTone,
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
		candidates, failure := search.positionCandidates(position, mora, previousVowel, phraseStart, previousLayer)
		if failure != nil {
			if missing == nil {
				return nil, failure
			}
			*missing = append(*missing, *failure)
		}
		layers = append(layers, candidates)
		previousLayer = candidates
		previousVowel = mora.Vowel
		phraseStart = false
	}
	return layers, nil
}

type candidateContext struct {
	bank          *Bank
	policy        AliasPolicy
	affix         Affix
	hasAffix      bool
	subbank       Subbank
	requestedTone string
	resolvedTone  string
}

func (c candidateContext) positionCandidates(position int, mora frontend.Mora, previousVowel string, phraseStart bool, previousLayer []Selection) ([]Selection, *MissingAliasError) {
	b, policy, affix, hasAffix := c.bank, c.policy, c.affix, c.hasAffix
	candidateSpecs := aliasCandidatesWithPolicy(mora.Text, previousVowel, phraseStart, policy)
	consonant := mora.Consonant
	if consonant == "" {
		consonant = frontend.ConsonantOf(mora.Text)
	}
	transitionSpecs := vcAliasCandidates(previousVowel, consonant, policy)
	explicitCandidates := mora.Aliases != nil && len(mora.Aliases.Main) > 0
	if explicitCandidates {
		candidateSpecs = explicitMainAliasCandidates(mora.Aliases.Main, mora.Aliases.MainKinds, mora.Text)
		transitionSpecs = explicitAliasCandidates(mora.Aliases.Transition, AliasVC)
	}
	var endingSpecs [][]aliasCandidate
	if mora.Aliases != nil {
		for _, aliases := range mora.Aliases.Endings {
			endingSpecs = append(endingSpecs, explicitAliasCandidates(aliases, AliasOther))
		}
	}
	if hasAffix {
		strictSubbank := c.subbank.ID != "" && c.subbank.ID != "prefix.map"
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
		for index := range endingSpecs {
			endingSpecs[index] = affixCandidatesWithFallback(endingSpecs[index], affix, true)
		}
	}
	if !explicitCandidates && previousVowel == "cl" && !hasUsableCandidateEntries(b, candidateSpecs) {
		// 促音後は無音から始まるため、単独音がなければ語頭形を使う。
		headSpecs := aliasCandidatesWithPolicy(mora.Text, "", true, policy)
		if hasAffix {
			headSpecs = affixCandidatesWithFallback(headSpecs, affix, true)
		}
		candidateSpecs = headSpecs
	}
	if !explicitCandidates {
		candidateSpecs = preferOriginalKanaCandidates(b, candidateSpecs)
	}
	endings := c.endingPlan(mora, endingSpecs)
	allSpecs := append(append([]aliasCandidate{}, candidateSpecs...), transitionSpecs...)
	for _, specs := range endings.specs {
		allSpecs = append(allSpecs, specs...)
	}
	builder := &positionBuilder{context: c, position: position, mora: mora, candidates: candidateNames(allSpecs), endings: endings}
	var candidatesAtPosition []Selection
	for _, candidate := range candidateSpecs {
		for _, validated := range builder.validatedEntries(candidate.name, b.Entries[candidate.name]) {
			main := builder.attachVariants(builder.selection(candidate, AliasKind(candidate.kind), validated))
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
				for _, validatedTransition := range builder.validatedEntries(transitionSpec.name, b.Entries[transitionSpec.name]) {
					transition := builder.selection(transitionSpec, AliasVC, validatedTransition)
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
	rejections := builder.rejections
	for index := range candidatesAtPosition {
		selected := &candidatesAtPosition[index]
		if mora.Aliases != nil && selected.Transition == nil {
			for alias, phones := range mora.Aliases.MainMissing {
				if selected.Alias == alias || (hasAffix && selected.Alias == affix.Prefix+alias+affix.Suffix) {
					selected.MissingPhones = append(append([]SpeechGap(nil), selected.MissingPhones...), SpeechGap{Position: position, Role: "onset", Phones: append([]string(nil), phones...), Aliases: append([]string(nil), mora.Aliases.Transition...)})
					break
				}
			}
		}
		selected.CandidateRejections = append([]CandidateRejection(nil), rejections...)
		if selected.Transition != nil {
			selected.Transition.CandidateRejections = append([]CandidateRejection(nil), rejections...)
		}
		for endingIndex := range selected.Endings {
			selected.Endings[endingIndex].CandidateRejections = append([]CandidateRejection(nil), rejections...)
		}
	}
	if len(candidatesAtPosition) == 0 {
		if mora.Vowel == "cl" {
			return []Selection{{
				Position: position, Mora: mora, Alias: "<closure>",
				Kind: AliasOther, FallbackTier: 0,
				Candidates: builder.candidates, CandidateCount: 1,
				TargetScore: 100,
			}}, nil
		}
		return nil, &MissingAliasError{Position: position, Mora: mora.Text, Candidates: builder.candidates, CandidateRejections: rejections}
	}
	applyCompositePreferences(candidatesAtPosition, policy)
	applyEnglishCandidatePreferences(candidatesAtPosition, previousLayer)
	candidatesAtPosition = pruneCandidates(candidatesAtPosition)
	for index := range candidatesAtPosition {
		candidatesAtPosition[index].CandidateCount = len(candidatesAtPosition)
		if candidatesAtPosition[index].Transition != nil {
			candidatesAtPosition[index].Transition.CandidateCount = len(candidatesAtPosition)
		}
		for endingIndex := range candidatesAtPosition[index].Endings {
			candidatesAtPosition[index].Endings[endingIndex].CandidateCount = len(candidatesAtPosition)
		}
	}
	return candidatesAtPosition, nil
}

// 元の候補は、語末子音の分割案を比較するために残す。
type endingPlan struct {
	specs, originalSpecs   [][]aliasCandidate
	phones, originalPhones [][]string
	starts, originalStarts []int
}

func (c candidateContext) endingPlan(mora frontend.Mora, endingSpecs [][]aliasCandidate) endingPlan {
	endingPhones := make([][]string, len(endingSpecs))
	endingStarts := make([]int, len(endingSpecs))
	codaStart := 0
	for i := range endingSpecs {
		endingStarts[i] = codaStart
		if mora.Aliases != nil && i < len(mora.Aliases.EndingPhones) {
			endingPhones[i] = mora.Aliases.EndingPhones[i]
			codaStart += len(endingPhones[i])
		}
	}
	plan := endingPlan{
		specs: endingSpecs, originalSpecs: endingSpecs,
		phones: endingPhones, originalPhones: endingPhones,
		starts: endingStarts, originalStarts: endingStarts,
	}
	if mora.Language != frontend.LanguageEnglish || mora.Aliases == nil {
		return plan
	}
	var expanded [][]aliasCandidate
	var phones [][]string
	var starts []int
	for i, specs := range endingSpecs {
		var chain []frontend.CodaAlias
		if i < len(mora.Aliases.EndingFallbacks) && !hasUsableCandidateEntries(c.bank, specs) {
			chain = selectCodaChain(c.bank, mora.Aliases.EndingFallbacks[i], c.affix, c.hasAffix)
		}
		if len(chain) > 0 {
			for _, edge := range chain {
				expanded = append(expanded, c.edgeCandidates(edge))
				phones = append(phones, edge.Phones)
				starts = append(starts, edge.CodaStart)
			}
		} else {
			expanded = append(expanded, specs)
			phones = append(phones, endingPhones[i])
			starts = append(starts, endingStarts[i])
		}
	}
	plan.specs, plan.phones, plan.starts = expanded, phones, starts
	return plan
}

func (c candidateContext) edgeCandidates(edge frontend.CodaAlias) []aliasCandidate {
	names := explicitAliasCandidates(edge.Aliases, AliasOther)
	if c.hasAffix {
		names = affixCandidatesWithFallback(names, c.affix, true)
	}
	return names
}

type validatedEntry struct {
	entry      oto.Entry
	validation EntryValidation
}

type positionBuilder struct {
	context    candidateContext
	position   int
	mora       frontend.Mora
	candidates []string
	endings    endingPlan
	rejections []CandidateRejection
}

func (p *positionBuilder) validatedEntries(alias string, entries []oto.Entry) []validatedEntry {
	valid := make([]validatedEntry, 0, len(entries))
	for _, entry := range entries {
		validation := p.context.bank.validateEntry(entry)
		if validation.Status == "unusable" {
			p.rejections = append(p.rejections, CandidateRejection{Alias: alias, Source: entry.Filename, Reason: validation.Reason})
			continue
		}
		valid = append(valid, validatedEntry{entry: entry, validation: validation})
	}
	return valid
}

func (p *positionBuilder) selection(spec aliasCandidate, kind AliasKind, validated validatedEntry) Selection {
	c := p.context
	return Selection{
		Position: p.position, Mora: p.mora, Alias: spec.name, Kind: kind,
		FallbackTier: spec.tier, Entry: validated.entry, Candidates: p.candidates,
		TargetScore: validatedCandidateScore(p.mora.Language, spec.tier, validated.entry, validated.validation),
		SubbankID:   c.subbank.ID, Color: c.subbank.Color, RequestedTone: c.requestedTone,
		ResolvedTone: c.resolvedTone, EntryStatus: validated.validation.Status, EntryValidation: validated.validation.Checks,
	}
}

func (p *positionBuilder) attachEndings(main Selection, endingSpecs [][]aliasCandidate, endingPhones [][]string, endingStarts []int) Selection {
	var endingLayers [][]Selection
	for endingIndex, specs := range endingSpecs {
		var choices []Selection
		for _, endingSpec := range specs {
			for _, validatedEnding := range p.validatedEntries(endingSpec.name, p.context.bank.Entries[endingSpec.name]) {
				ending := p.selection(endingSpec, AliasOther, validatedEnding)
				ending.CodaPhones = append([]string(nil), endingPhones[endingIndex]...)
				ending.CodaStart = endingStarts[endingIndex]
				ending.EndingIndex = endingIndex
				choices = append(choices, ending)
			}
		}
		if len(choices) == 0 {
			if len(endingPhones[endingIndex]) > 0 {
				gap := SpeechGap{Position: p.position, Role: "coda", Phones: append([]string(nil), endingPhones[endingIndex]...)}
				for _, spec := range specs {
					gap.Aliases = append(gap.Aliases, spec.name)
				}
				main.MissingPhones = append(main.MissingPhones, gap)
			}
			// 録音のない末子音で、後続の録音可能な子音を隠さない。
			continue
		}
		endingLayers = append(endingLayers, choices)
	}
	for _, choices := range endingLayers {
		best := choices[0]
		for _, choice := range choices[1:] {
			if choice.TargetScore > best.TargetScore {
				best = choice
			}
		}
		main.Endings = append(main.Endings, best)
	}
	if p.mora.Language == frontend.LanguageEnglish {
		main.EndingCandidates = endingLayers
	}
	return main
}

func (p *positionBuilder) attachVariants(main Selection) Selection {
	e := p.endings
	base := p.attachEndings(main, e.specs, e.phones, e.starts)
	if p.mora.Language != frontend.LanguageEnglish || p.mora.Aliases == nil {
		return base
	}
	c := p.context
	for group, edges := range p.mora.Aliases.EndingFallbacks {
		if group >= len(e.originalSpecs) {
			continue
		}
		for _, chain := range selectCodaChains(c.bank, edges, c.affix, c.hasAffix) {
			specs := append([][]aliasCandidate(nil), e.originalSpecs[:group]...)
			phones := append([][]string(nil), e.originalPhones[:group]...)
			starts := append([]int(nil), e.originalStarts[:group]...)
			for _, edge := range chain {
				specs = append(specs, c.edgeCandidates(edge))
				phones = append(phones, edge.Phones)
				starts = append(starts, edge.CodaStart)
			}
			specs = append(specs, e.originalSpecs[group+1:]...)
			phones = append(phones, e.originalPhones[group+1:]...)
			starts = append(starts, e.originalStarts[group+1:]...)
			variant := p.attachEndings(main, specs, phones, starts)
			if missingPhoneCount(variant.MissingPhones) <= missingPhoneCount(base.MissingPhones) {
				base.EndingAlternatives = append(base.EndingAlternatives, variant)
			}
		}
	}
	return base
}

func missingPhoneCount(gaps []SpeechGap) int {
	count := 0
	for _, gap := range gaps {
		count += len(gap.Phones)
	}
	return count
}

func candidateScore(language string, candidateTier int, entry oto.Entry) float64 {
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
	if entry.Overlap <= entry.Preutterance || language == frontend.LanguageEnglish {
		// C+V/VCCVの母音はoverlapがpreutteranceを超えても正常。減点すると文頭形が優先される。
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

func validatedCandidateScore(language string, candidateTier int, entry oto.Entry, validation EntryValidation) float64 {
	score := candidateScore(language, candidateTier, entry)
	if validation.Status == "degraded" {
		score -= 3
	}
	return score
}

func localCandidateScore(candidate Selection) float64 {
	return candidate.TargetScore + candidate.PreferenceScore
}

// 上限超過時だけ絞り込み、別録音の候補も残す。上限以下では順序を変えない。
func pruneCandidates(candidates []Selection) []Selection {
	if len(candidates) <= maxCandidatesPerPosition {
		return candidates
	}
	ranked := make([]Selection, len(candidates))
	copy(ranked, candidates)
	sort.SliceStable(ranked, func(i, j int) bool {
		return localCandidateScore(ranked[i]) > localCandidateScore(ranked[j])
	})

	limit := maxCandidatesPerPosition
	reserve := min(minDistinctSourceCandidates, limit)
	selected := make([]Selection, 0, limit)
	chosen := make([]bool, len(ranked))

	for index := 0; index < len(ranked) && len(selected) < limit-reserve; index++ {
		selected = append(selected, ranked[index])
		chosen[index] = true
	}

	sources := make(map[string]bool, limit)
	for _, candidate := range selected {
		sources[candidate.Entry.Filename] = true
	}
	for index := 0; index < len(ranked) && len(selected) < limit; index++ {
		if chosen[index] {
			continue
		}
		filename := ranked[index].Entry.Filename
		if filename == "" || sources[filename] {
			continue
		}
		sources[filename] = true
		selected = append(selected, ranked[index])
		chosen[index] = true
	}

	for index := 0; index < len(ranked) && len(selected) < limit; index++ {
		if chosen[index] {
			continue
		}
		selected = append(selected, ranked[index])
		chosen[index] = true
	}
	return selected
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

func explicitMainAliasCandidates(names, kinds []string, fallback string) []aliasCandidate {
	result := explicitAliasCandidates(names, AliasOther)
	for index := range result {
		if index < len(kinds) {
			switch strings.ToLower(strings.TrimSpace(kinds[index])) {
			case "cv":
				result[index].kind = AliasCV
			case "vcv":
				result[index].kind = AliasVCV
			case "vc":
				result[index].kind = AliasVC
			}
		}
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

// 専用録音がない場合の代替読み。一般的な近似に限る。
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
	// 専用録音がない外来音は、日本語の近い発音で代替する。
	case "ゔ":
		return []string{"ぶ"}
	case "ゔぁ":
		return []string{"ば"}
	case "ゔぃ":
		return []string{"び"}
	case "ゔぇ":
		return []string{"べ"}
	case "ゔぉ":
		return []string{"ぼ"}
	case "ゔゅ":
		return []string{"びゅ"}
	case "でゅ":
		return []string{"じゅ"}
	case "てゅ":
		return []string{"ちゅ"}
	}
	return nil
}

// fallbackは同音候補への追加減点。
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
	// 同音候補より元の仮名を優先する。
	base := []aliasForm{{text: mora}}
	for _, equivalent := range equivalentKanaForms(mora) {
		base = append(base, aliasForm{text: equivalent, fallback: 1, equivalent: true})
	}
	for _, form := range base {
		forms = append(forms, form)
		if katakana := toKatakana(form.text); katakana != form.text {
			forms = append(forms, aliasForm{text: katakana, fallback: form.fallback, equivalent: form.equivalent})
		}
		// ヴ行は「ヴぁ」のように、ヴだけ片仮名で小書きは平仮名の表記の音源が多い。
		if rest, ok := strings.CutPrefix(form.text, "ゔ"); ok && rest != "" {
			forms = append(forms, aliasForm{text: "ヴ" + rest, fallback: form.fallback, equivalent: form.equivalent})
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
