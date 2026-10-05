package voicebank

import (
	"fmt"
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
