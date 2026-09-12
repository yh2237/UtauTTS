package voicebank

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"utautts/internal/connection"
	"utautts/internal/frontend"
	"utautts/internal/oto"
)

type LatticeAudit struct {
	Tone                    string          `json:"tone,omitempty"`
	Color                   string          `json:"color,omitempty"`
	VCVSelectedPositions    int             `json:"vcv_selected_positions"`
	CVVCSelectedPositions   int             `json:"cvvc_selected_positions"`
	CVSelectedPositions     int             `json:"cv_selected_positions"`
	Positions               []PositionAudit `json:"positions"`
	MultiCandidatePositions int             `json:"multi_candidate_positions"`
	SameTargetPositions     int             `json:"same_target_positions"`
	PitchCandidates         int             `json:"pitch_candidates"`
	VoicedCandidates        int             `json:"voiced_candidates"`
	PitchChoicePositions    int             `json:"pitch_choice_positions"`
	WidePitchPositions      int             `json:"wide_pitch_positions"`
	WidePitchWithinGroup    int             `json:"wide_pitch_within_group_positions"`
}

type PositionAudit struct {
	VCVCandidateCount    int              `json:"vcv_candidate_count"`
	CVVCCandidateCount   int              `json:"cvvc_candidate_count"`
	CVCandidateCount     int              `json:"cv_candidate_count"`
	SelectedAliasKind    string           `json:"selected_alias_kind"`
	SelectedAlias        string           `json:"selected_alias"`
	SelectedComposite    bool             `json:"selected_composite,omitempty"`
	SelectedTransition   string           `json:"selected_transition,omitempty"`
	SelectedFallbackTier int              `json:"selected_fallback_tier"`
	SelectedSubbankID    string           `json:"selected_subbank_id,omitempty"`
	SelectedColor        string           `json:"selected_color,omitempty"`
	RequestedTone        string           `json:"requested_tone,omitempty"`
	ResolvedTone         string           `json:"resolved_tone,omitempty"`
	Position             int              `json:"position"`
	Mora                 string           `json:"mora"`
	CandidateCount       int              `json:"candidate_count"`
	TargetRange          float64          `json:"target_range"`
	VoicedCandidateCount int              `json:"voiced_candidate_count"`
	PitchMinHz           float64          `json:"pitch_min_hz,omitempty"`
	PitchMaxHz           float64          `json:"pitch_max_hz,omitempty"`
	PitchSpanCents       float64          `json:"pitch_span_cents,omitempty"`
	WithinGroupPitchSpan float64          `json:"within_group_pitch_span_cents,omitempty"`
	SourceGroupCount     int              `json:"source_group_count"`
	Candidates           []CandidateAudit `json:"candidates"`
}

type CandidateAudit struct {
	Index               int                  `json:"index"`
	Alias               string               `json:"alias"`
	AliasKind           string               `json:"alias_kind"`
	Composite           bool                 `json:"composite,omitempty"`
	TransitionAlias     string               `json:"transition_alias,omitempty"`
	FallbackTier        int                  `json:"fallback_tier"`
	Source              string               `json:"source"`
	OtoPath             string               `json:"oto_path"`
	OtoLine             int                  `json:"oto_line"`
	TargetScore         float64              `json:"target_score"`
	SubbankID           string               `json:"subbank_id,omitempty"`
	Color               string               `json:"color,omitempty"`
	EntryStatus         string               `json:"entry_status,omitempty"`
	EntryValidation     []string             `json:"entry_validation,omitempty"`
	CandidateRejections []CandidateRejection `json:"candidate_rejections,omitempty"`
	SourceGroup         string               `json:"source_group"`
	SourceF0Hz          float64              `json:"source_f0_hz"`
	PitchValid          bool                 `json:"pitch_valid"`
	BestHandcraftedJoin float64              `json:"best_handcrafted_join"`
}

// AuditLatticeは診断用に全候補と各候補への最良の入エッジを返す。
func (b *Bank) AuditLattice(morae []frontend.Mora, tone string) (*LatticeAudit, error) {
	return b.AuditLatticeWithConfig(morae, ResolveConfig{Tone: tone})
}

func (b *Bank) AuditLatticeWithConfig(morae []frontend.Mora, cfg ResolveConfig) (*LatticeAudit, error) {
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
	selectedByPosition := selectionsByPosition(selectBestPaths(layers, b.extractor))
	cache := connection.NewExtractor()
	pitchCache := map[candidatePitchKey]candidatePitch{}
	result := &LatticeAudit{Tone: cfg.Tone, Color: cfg.Color}
	for layerIndex, layer := range layers {
		if len(layer) == 0 {
			continue
		}
		position := PositionAudit{
			Position: layer[0].Position, Mora: layer[0].Mora.Text,
			CandidateCount: len(layer),
		}
		selected := selectedByPosition[layer[0].Position]
		position.SelectedAlias = selected.Alias
		position.SelectedAliasKind = string(selected.Kind)
		position.SelectedComposite = selected.Composite
		if selected.Transition != nil {
			position.SelectedTransition = selected.Transition.Alias
		}
		position.SelectedFallbackTier = selected.FallbackTier
		position.SelectedSubbankID = selected.SubbankID
		position.SelectedColor = selected.Color
		position.RequestedTone = selected.RequestedTone
		position.ResolvedTone = selected.ResolvedTone
		minimum, maximum := layer[0].TargetScore, layer[0].TargetScore
		sourceGroups := map[string]bool{}
		groupPitches := map[string][]float64{}
		for candidateIndex, candidate := range layer {
			switch candidate.Kind {
			case AliasVCV:
				position.VCVCandidateCount++
			case AliasCV:
				position.CVCandidateCount++
			}
			if candidate.Composite {
				position.CVVCCandidateCount++
			}
			minimum = min(minimum, candidate.TargetScore)
			maximum = max(maximum, candidate.TargetScore)
			measuredPitch := measureCandidateF0(candidate.Entry, pitchCache)
			if candidate.Entry.Filename != "" {
				result.PitchCandidates++
			}
			if measuredPitch.Valid {
				result.VoicedCandidates++
				position.VoicedCandidateCount++
				if position.PitchMinHz == 0 || measuredPitch.Hz < position.PitchMinHz {
					position.PitchMinHz = measuredPitch.Hz
				}
				position.PitchMaxHz = max(position.PitchMaxHz, measuredPitch.Hz)
			}
			group := sourceGroup(b.Root, candidate.Entry)
			sourceGroups[group] = true
			if measuredPitch.Valid {
				groupPitches[group] = append(groupPitches[group], measuredPitch.Hz)
			}
			audit := CandidateAudit{
				Index: candidateIndex, Alias: candidate.Alias, AliasKind: string(candidate.Kind), FallbackTier: candidate.FallbackTier,
				Composite: candidate.Composite,
				Source:    relativePath(b.Root, candidate.Entry.Filename),
				OtoPath:   relativePath(b.Root, candidate.Entry.OtoPath), OtoLine: candidate.Entry.Line,
				TargetScore: candidate.TargetScore, SourceGroup: group,
				SubbankID: candidate.SubbankID, Color: candidate.Color,
				EntryStatus: candidate.EntryStatus, EntryValidation: append([]string(nil), candidate.EntryValidation...),
				CandidateRejections: append([]CandidateRejection(nil), candidate.CandidateRejections...),
				SourceF0Hz:          measuredPitch.Hz, PitchValid: measuredPitch.Valid,
			}
			if candidate.Transition != nil {
				audit.TransitionAlias = candidate.Transition.Alias
			}
			if layerIndex > 0 && len(layers[layerIndex-1]) > 0 {
				incoming := candidate.Entry
				if candidate.Transition != nil {
					incoming = candidate.Transition.Entry
				}
				audit.BestHandcraftedJoin = bestIncoming(layers[layerIndex-1], incoming, cache).score
			}
			position.Candidates = append(position.Candidates, audit)
		}
		position.TargetRange = maximum - minimum
		position.SourceGroupCount = len(sourceGroups)
		if position.VoicedCandidateCount > 1 && position.PitchMinHz > 0 {
			position.PitchSpanCents = 1200 * math.Log2(position.PitchMaxHz/position.PitchMinHz)
			result.PitchChoicePositions++
			if position.PitchSpanCents >= 50 {
				result.WidePitchPositions++
			}
		}
		for _, values := range groupPitches {
			if len(values) < 2 {
				continue
			}
			minimum, maximum := values[0], values[0]
			for _, value := range values[1:] {
				minimum = min(minimum, value)
				maximum = max(maximum, value)
			}
			span := 1200 * math.Log2(maximum/minimum)
			position.WithinGroupPitchSpan = max(position.WithinGroupPitchSpan, span)
		}
		if position.WithinGroupPitchSpan >= 50 {
			result.WidePitchWithinGroup++
		}
		if len(layer) > 1 {
			result.MultiCandidatePositions++
			if position.TargetRange == 0 {
				result.SameTargetPositions++
			}
		}
		switch position.SelectedAliasKind {
		case string(AliasVCV):
			result.VCVSelectedPositions++
		case string(AliasCV):
			if position.SelectedComposite {
				result.CVVCSelectedPositions++
			} else {
				result.CVSelectedPositions++
			}
		}
		result.Positions = append(result.Positions, position)
	}
	return result, nil
}

func sourceGroup(root string, entry oto.Entry) string {
	if entry.SourceGroup != "" {
		return entry.SourceGroup
	}
	relative := relativePath(root, entry.OtoPath)
	directory := filepath.ToSlash(filepath.Dir(relative))
	if directory == "." || directory == "" {
		return "root"
	}
	return strings.Split(directory, "/")[0]
}

type incomingScore struct{ score float64 }

func bestIncoming(previous []Selection, current oto.Entry, cache *connection.Extractor) incomingScore {
	best := incomingScore{score: -1e100}
	for _, candidate := range previous {
		score := joinScore(candidate.Entry, current, cache)
		if score > best.score {
			best = incomingScore{score: score}
		}
	}
	return best
}

func selectionsByPosition(selections []Selection) map[int]Selection {
	result := make(map[int]Selection, len(selections))
	for _, selection := range selections {
		result[selection.Position] = selection
	}
	return result
}

func relativePath(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}
