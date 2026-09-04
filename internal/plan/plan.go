package plan

import (
	"fmt"
	"math"
	"strings"

	"utautts/internal/frontend"
	"utautts/internal/oto"
	"utautts/internal/prosody"
	"utautts/internal/voicebank"
)

const Version = 19

type Config struct {
	MoraDurationMS   float64
	PauseDurationMS  float64
	MoraDurationsMS  []float64
	Predictions      []prosody.Prediction
	Tone             string
	Color            string
	AcousticMode     string
	SelectionMode    voicebank.SelectionMode
	AliasPolicy      voicebank.AliasPolicy
	JoinCostMode     string
	JoinModelVersion int
	JoinScoreScale   float64
}

type Plan struct {
	Version                 int                      `json:"version"`
	Voicebank               string                   `json:"voicebank"`
	Text                    string                   `json:"text,omitempty"`
	Reading                 string                   `json:"reading"`
	Language                string                   `json:"language,omitempty"`
	Phonemizer              string                   `json:"phonemizer,omitempty"`
	Tone                    string                   `json:"tone,omitempty"`
	Color                   string                   `json:"color,omitempty"`
	AcousticMode            string                   `json:"acoustic_mode,omitempty"`
	SelectionMode           string                   `json:"selection_mode"`
	AliasPolicy             string                   `json:"alias_policy"`
	RequestedAliasPolicy    string                   `json:"requested_alias_policy,omitempty"`
	JoinCostMode            string                   `json:"join_cost_mode"`
	JoinModelVersion        int                      `json:"join_model_version,omitempty"`
	JoinScoreScale          float64                  `json:"join_score_scale,omitempty"`
	BoundaryBridgeMS        float64                  `json:"boundary_bridge_ms,omitempty"`
	BoundaryBridgeThreshold float64                  `json:"boundary_bridge_threshold,omitempty"`
	BoundaryBridges         []BoundaryBridge         `json:"boundary_bridges,omitempty"`
	BoundaryRepairDecisions []BoundaryRepairDecision `json:"boundary_repair_decisions,omitempty"`
	CVVCTiming              string                   `json:"cvvc_timing,omitempty"`
	CVVCTransitionGain      float64                  `json:"cvvc_transition_gain,omitempty"`
	CVVCPreBoundaryFade     bool                     `json:"cvvc_pre_boundary_fade,omitempty"`
	LeadingMarginMS         float64                  `json:"leading_margin_ms,omitempty"`
	DurationMS              float64                  `json:"duration_ms"`
	Units                   []Unit                   `json:"units"`
	Morae                   []frontend.Mora          `json:"-"`
}

// BoundaryBridgeはレンダラーが適用する短い遷移補正を記録する。
type BoundaryBridge struct {
	UnitIndex   int     `json:"unit_index"`
	Position    int     `json:"position"`
	StartMS     float64 `json:"start_ms"`
	EndMS       float64 `json:"end_ms"`
	DurationMS  float64 `json:"duration_ms"`
	LagMS       float64 `json:"lag_ms,omitempty"`
	JoinScore   float64 `json:"join_score"`
	Correlation float64 `json:"correlation,omitempty"`
	Source      string  `json:"source"`
	Kind        string  `json:"kind"`
}

// BoundaryRepairDecisionは通常接続と補正接続の選択結果を記録する。
type BoundaryRepairDecision struct {
	UnitIndex        int     `json:"unit_index"`
	Position         int     `json:"position"`
	CandidateCount   int     `json:"candidate_count"`
	SelectedKind     string  `json:"selected_kind"`
	Applied          bool    `json:"applied"`
	DurationMS       float64 `json:"duration_ms,omitempty"`
	LagMS            float64 `json:"lag_ms,omitempty"`
	JoinScore        float64 `json:"join_score"`
	Correlation      float64 `json:"correlation,omitempty"`
	BaselinePeak     float64 `json:"baseline_peak_delta"`
	SelectedPeak     float64 `json:"selected_peak_delta"`
	BaselineDeltaRMS float64 `json:"baseline_delta_rms"`
	SelectedDeltaRMS float64 `json:"selected_delta_rms"`
}

type Unit struct {
	Position                  int                            `json:"position"`
	Role                      string                         `json:"role"`
	ParentPosition            int                            `json:"parent_position,omitempty"`
	TransitionFrom            string                         `json:"transition_from,omitempty"`
	TransitionTo              string                         `json:"transition_to,omitempty"`
	Mora                      string                         `json:"mora"`
	Alias                     string                         `json:"alias"`
	Source                    string                         `json:"source"`
	Silent                    bool                           `json:"silent,omitempty"`
	LongUnitGroup             int                            `json:"long_unit_group,omitempty"`
	LongUnitSize              int                            `json:"long_unit_size,omitempty"`
	OtoPath                   string                         `json:"oto_path"`
	OtoLine                   int                            `json:"oto_line"`
	NoteStartMS               float64                        `json:"note_start_ms"`
	DurationMS                float64                        `json:"duration_ms"`
	OffsetMS                  float64                        `json:"offset_ms"`
	ConsonantMS               float64                        `json:"consonant_ms"`
	CutoffMS                  float64                        `json:"cutoff_ms"`
	PreutteranceMS            float64                        `json:"preutterance_ms"`
	OverlapMS                 float64                        `json:"overlap_ms"`
	PitchFactor               float64                        `json:"pitch_factor"`
	EnergyFactor              float64                        `json:"energy_factor"`
	ResamplerVelocity         int                            `json:"resampler_velocity,omitempty"`
	ResamplerVolume           int                            `json:"resampler_volume,omitempty"`
	ResamplerFlags            string                         `json:"resampler_flags,omitempty"`
	ResamplerModulation       int                            `json:"resampler_modulation,omitempty"`
	ResamplerTempo            float64                        `json:"resampler_tempo,omitempty"`
	TimingScale               float64                        `json:"timing_scale"`
	EffectivePreutteranceMS   float64                        `json:"effective_preutterance_ms"`
	EffectiveConsonantMS      float64                        `json:"effective_consonant_ms"`
	EffectiveOverlapMS        float64                        `json:"effective_overlap_ms"`
	SourceF0Hz                float64                        `json:"source_f0_hz,omitempty"`
	TargetF0Hz                float64                        `json:"target_f0_hz,omitempty"`
	IntonationFactor          float64                        `json:"intonation_factor"`
	CandidateCount            int                            `json:"candidate_count"`
	TargetScore               float64                        `json:"target_score"`
	JoinScore                 float64                        `json:"join_score"`
	JoinProbability           float64                        `json:"join_probability,omitempty"`
	TransitionJoinScore       float64                        `json:"transition_join_score,omitempty"`
	TransitionJoinProbability float64                        `json:"transition_join_probability,omitempty"`
	PathScore                 float64                        `json:"path_score"`
	AliasKind                 string                         `json:"alias_kind,omitempty"`
	FallbackTier              int                            `json:"fallback_tier"`
	SubbankID                 string                         `json:"subbank_id,omitempty"`
	Color                     string                         `json:"color,omitempty"`
	RequestedTone             string                         `json:"requested_tone,omitempty"`
	ResolvedTone              string                         `json:"resolved_tone,omitempty"`
	EntryStatus               string                         `json:"entry_status,omitempty"`
	EntryValidation           []string                       `json:"entry_validation,omitempty"`
	CandidateRejections       []voicebank.CandidateRejection `json:"candidate_rejections,omitempty"`
	AcousticTargetScore       float64                        `json:"acoustic_target_score,omitempty"`
	AcousticJoinScore         float64                        `json:"acoustic_join_score,omitempty"`
	SelectionMargin           float64                        `json:"selection_margin,omitempty"`
}

func Build(bank *voicebank.Bank, reading string, morae []frontend.Mora, selections []voicebank.Selection, cfg Config) (*Plan, error) {
	if math.IsNaN(cfg.MoraDurationMS) || math.IsInf(cfg.MoraDurationMS, 0) {
		return nil, fmt.Errorf("mora duration must be finite, got %v", cfg.MoraDurationMS)
	}
	if math.IsNaN(cfg.PauseDurationMS) || math.IsInf(cfg.PauseDurationMS, 0) {
		return nil, fmt.Errorf("pause duration must be finite, got %v", cfg.PauseDurationMS)
	}
	if cfg.MoraDurationMS <= 0 {
		cfg.MoraDurationMS = 140
	}
	if cfg.PauseDurationMS <= 0 {
		cfg.PauseDurationMS = 180
	}
	for index, duration := range cfg.MoraDurationsMS {
		if math.IsNaN(duration) || math.IsInf(duration, 0) {
			return nil, fmt.Errorf("mora duration at position %d must be finite, got %v", index, duration)
		}
	}
	byPosition := make(map[int]voicebank.Selection, len(selections))
	for _, selection := range selections {
		byPosition[selection.Position] = selection
	}

	selectionMode := cfg.SelectionMode
	if selectionMode == "" {
		selectionMode = voicebank.SelectionViterbi
	}
	aliasPolicy := cfg.AliasPolicy
	if aliasPolicy == "" {
		aliasPolicy = voicebank.AliasPolicyAuto
	}
	joinCostMode := cfg.JoinCostMode
	if joinCostMode == "" {
		joinCostMode = "handcrafted"
	}
	result := &Plan{
		Version: Version, Voicebank: bank.Root, Reading: reading,
		Morae: append([]frontend.Mora(nil), morae...),
		Tone:  cfg.Tone, Color: cfg.Color, AcousticMode: cfg.AcousticMode,
		SelectionMode: string(selectionMode), AliasPolicy: string(aliasPolicy), JoinCostMode: joinCostMode,
		JoinModelVersion: cfg.JoinModelVersion,
		JoinScoreScale:   cfg.JoinScoreScale,
	}
	cursor := 0.0
	for position, mora := range morae {
		prediction := prosody.Prediction{PitchFactor: 1, EnergyFactor: 1}
		if position < len(cfg.Predictions) {
			prediction = cfg.Predictions[position]
			if prediction.PitchFactor <= 0 {
				prediction.PitchFactor = 1
			}
			if prediction.EnergyFactor <= 0 {
				prediction.EnergyFactor = 1
			}
		}
		if mora.Pause {
			duration, manuallySet := configuredMoraDuration(position, cfg)
			if !manuallySet {
				duration = cfg.PauseDurationMS
				if prediction.DurationMS > 0 {
					duration = prediction.DurationMS
				} else if prediction.DurationFactor > 0 {
					duration *= prediction.DurationFactor
				}
			}
			cursor += duration
			continue
		}
		selection, ok := byPosition[position]
		if !ok {
			return nil, fmt.Errorf("selection missing for mora %q at position %d", mora.Text, position)
		}
		duration, manuallySet := configuredMoraDuration(position, cfg)
		if !manuallySet {
			duration = durationFor(mora, cfg.MoraDurationMS)
			if prediction.DurationMS > 0 {
				duration = prediction.DurationMS
			} else if prediction.DurationFactor > 0 {
				duration *= prediction.DurationFactor
			}
		}
		if selection.Transition != nil {
			transition := selection.Transition
			transitionDuration := transitionDurationFor(transition.Entry, duration)
			result.Units = append(result.Units, unitFromSelection(transition, position, cursor, transitionDuration, prediction, "transition"))
		}
		aliasKind := selection.Kind
		if aliasKind == "" {
			aliasKind = voicebank.ClassifyAlias(selection.Alias)
		}
		mainUnit := unitFromSelection(&selection, position, cursor, duration, prediction, "mora")
		mainUnit.AliasKind = string(aliasKind)
		mainUnit.TransitionJoinScore = selection.TransitionJoinScore
		mainUnit.TransitionJoinProbability = selection.TransitionJoinProbability
		result.Units = append(result.Units, mainUnit)
		if len(selection.Endings) > 0 {
			endingDuration := endingDurationFor(duration, len(selection.Endings))
			endingStart := cursor + duration - endingDuration*float64(len(selection.Endings))
			for index := range selection.Endings {
				result.Units = append(result.Units, unitFromSelection(&selection.Endings[index], position, endingStart+float64(index)*endingDuration, endingDuration, prediction, "ending"))
			}
		}
		cursor += duration
	}
	result.DurationMS = cursor
	return result, nil
}

func unitFromSelection(selection *voicebank.Selection, position int, noteStart, duration float64, prediction prosody.Prediction, role string) Unit {
	entry := selection.Entry
	aliasKind := selection.Kind
	if aliasKind == "" {
		aliasKind = voicebank.ClassifyAlias(selection.Alias)
	}
	unit := Unit{
		Position:            position,
		Role:                role,
		Mora:                selection.Mora.Text,
		Alias:               selection.Alias,
		AliasKind:           string(aliasKind),
		FallbackTier:        selection.FallbackTier,
		SubbankID:           selection.SubbankID,
		Color:               selection.Color,
		RequestedTone:       selection.RequestedTone,
		ResolvedTone:        selection.ResolvedTone,
		EntryStatus:         selection.EntryStatus,
		EntryValidation:     append([]string(nil), selection.EntryValidation...),
		CandidateRejections: append([]voicebank.CandidateRejection(nil), selection.CandidateRejections...),
		AcousticTargetScore: selection.AcousticTargetScore,
		AcousticJoinScore:   selection.AcousticJoinScore,
		SelectionMargin:     selection.SelectionMargin,
		Source:              entry.Filename,
		Silent:              entry.Filename == "",
		OtoPath:             entry.OtoPath,
		OtoLine:             entry.Line,
		NoteStartMS:         noteStart,
		DurationMS:          duration,
		OffsetMS:            entry.Offset,
		ConsonantMS:         entry.Fixed,
		CutoffMS:            entry.Blank,
		PreutteranceMS:      entry.Preutterance,
		OverlapMS:           entry.Overlap,
		PitchFactor:         prediction.PitchFactor,
		EnergyFactor:        prediction.EnergyFactor,
		CandidateCount:      selection.CandidateCount,
		TargetScore:         selection.TargetScore,
		JoinScore:           selection.JoinScore,
		JoinProbability:     selection.JoinProbability,
		PathScore:           selection.PathScore,
	}
	if role == "transition" {
		unit.ParentPosition = position
		unit.TransitionFrom = transitionContext(selection.Alias)
		unit.TransitionTo = transitionTarget(selection.Alias)
		unit.PitchFactor = 1
		unit.EnergyFactor = 1
		unit.PreutteranceMS, unit.OverlapMS = transitionTiming(entry, duration)
		unit.ConsonantMS = math.Min(math.Max(0, entry.Fixed), duration)
	} else if role == "ending" {
		unit.ParentPosition = position
	}
	return unit
}

func endingDurationFor(moraDuration float64, count int) float64 {
	if count <= 0 {
		return 0
	}
	target := math.Max(12, math.Min(60, moraDuration/6))
	return math.Min(target, moraDuration*0.5/float64(count))
}

func transitionDurationFor(entry oto.Entry, moraDuration float64) float64 {
	base := entry.Preutterance - math.Min(math.Max(0, entry.Overlap), math.Max(0, entry.Preutterance))
	if base <= 0 {
		base = 35
	}
	maximum := math.Max(24, moraDuration*0.45)
	return math.Max(12, math.Min(maximum, base))
}

func transitionTiming(entry oto.Entry, duration float64) (preutterance, overlap float64) {
	preutterance = math.Max(0, entry.Preutterance)
	if preutterance <= 0 {
		preutterance = duration
	}
	overlap = math.Max(0, math.Min(entry.Overlap, preutterance))
	return preutterance, overlap
}

func transitionContext(alias string) string {
	parts := strings.Fields(alias)
	if len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func transitionTarget(alias string) string {
	parts := strings.Fields(alias)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func configuredMoraDuration(position int, cfg Config) (float64, bool) {
	if position < 0 || position >= len(cfg.MoraDurationsMS) {
		return 0, false
	}
	duration := cfg.MoraDurationsMS[position]
	if duration <= 0 {
		return 0, false
	}
	return duration, true
}

func durationFor(mora frontend.Mora, base float64) float64 {
	if mora.DurationScale > 0 {
		return base * mora.DurationScale
	}
	switch mora.Vowel {
	case "cl":
		return base * 0.65
	case "n":
		return base * 0.9
	}
	if mora.Text == "ー" {
		return base * 1.2
	}
	return base
}
