package voicebank

import (
	"strings"

	"github.com/yh2237/UtauTTS/internal/frontend"
	"github.com/yh2237/UtauTTS/internal/oto"
)

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
	if len(candidatesAtPosition) == 0 && !explicitCandidates {
		candidatesAtPosition = builder.sameVowelContinuations(previousVowel)
	}
	rejections := builder.rejections
	for index := range candidatesAtPosition {
		selected := &candidatesAtPosition[index]
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
	return plan
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
	return main
}

func (p *positionBuilder) attachVariants(main Selection) Selection {
	e := p.endings
	return p.attachEndings(main, e.specs, e.phones, e.starts)
}

const (
	steadyVowelDelayMs   = 80  // 母音の始まりから安定部分までの時間
	steadyVowelLeadMs    = 60  // 安定部分より前から読む長さ。先行発声と固定範囲に使う
	steadyVowelOverlapMs = 40  // 直前の同じ母音と重ねる長さ
	steadyVowelMinMs     = 120 // 切り出した後に残す最短の長さ
)

// 同じ母音の連続音（a あ）も単独音もない音源では、同じ母音の別の原音から安定部分を切り出してつなぐ。
// 語頭形は無音から始まり、句中で途切れて聞こえるため使わない。
func (p *positionBuilder) sameVowelContinuations(previousVowel string) []Selection {
	c := p.context
	vowelKana := map[string]string{"a": "あ", "i": "い", "u": "う", "e": "え", "o": "お"}[previousVowel]
	if vowelKana == "" || p.mora.Vowel != previousVowel || (p.mora.Text != "ー" && p.mora.Text != vowelKana) {
		return nil
	}
	donors := aliasCandidatesWithPolicy(vowelKana, "", true, c.policy)
	for _, context := range []string{"a", "i", "u", "e", "o", "n"} {
		donors = append(donors, aliasCandidatesWithPolicy(vowelKana, context, false, c.policy)...)
	}
	name := previousVowel + " " + vowelKana
	if c.hasAffix {
		donors = affixCandidatesWithFallback(donors, c.affix, true)
		name = c.affix.Prefix + name + c.affix.Suffix
	}
	// 後段は別名から母音どうしのつなぎとして扱う。
	spec := aliasCandidate{name: name, tier: 2, kind: AliasVCV}
	var result []Selection
	for _, donor := range uniqueCandidates(donors) {
		for _, entry := range c.bank.Entries[donor.name] {
			derived, ok := steadyVowelEntry(entry, name)
			if !ok {
				continue
			}
			for _, validated := range p.validatedEntries(name, []oto.Entry{derived}) {
				result = append(result, p.attachVariants(p.selection(spec, AliasVCV, validated)))
			}
		}
	}
	return result
}

func steadyVowelEntry(entry oto.Entry, alias string) (oto.Entry, bool) {
	offset := entry.Offset + entry.Preutterance + steadyVowelDelayMs - steadyVowelLeadMs
	derived := entry
	derived.Alias = alias
	derived.Offset = offset
	derived.Fixed = steadyVowelLeadMs
	derived.Preutterance = steadyVowelLeadMs
	derived.Overlap = steadyVowelOverlapMs
	// 負の除外範囲はoffsetからの長さなので、元の終わりを保つように付け直す。
	if entry.Blank < 0 {
		end := entry.Offset - entry.Blank
		if end-offset < steadyVowelMinMs {
			return oto.Entry{}, false
		}
		derived.Blank = offset - end
	}
	return derived, true
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
	case "ふゅ":
		return []string{"ひゅ"}
	case "くぁ", "くゎ":
		return []string{"か"}
	case "くぃ":
		return []string{"き"}
	case "くぇ":
		return []string{"け"}
	case "くぉ":
		return []string{"こ"}
	case "ぐぁ", "ぐゎ":
		return []string{"が"}
	case "ぐぃ":
		return []string{"ぎ"}
	case "ぐぇ":
		return []string{"げ"}
	case "ぐぉ":
		return []string{"ご"}
	case "きぇ":
		return []string{"け"}
	case "ぎぇ":
		return []string{"げ"}
	case "にぇ":
		return []string{"ね"}
	case "ひぇ":
		return []string{"へ"}
	case "びぇ":
		return []string{"べ"}
	case "ぴぇ":
		return []string{"ぺ"}
	case "みぇ":
		return []string{"め"}
	case "りぇ":
		return []string{"れ"}
	}
	return nil
}

// fallbackは同音候補への追加減点。
type aliasForm struct {
	text       string
	fallback   int
	equivalent bool
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
