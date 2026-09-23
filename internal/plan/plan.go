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

const Version = 26

// DefaultMoraDurationMSとDefaultPauseDurationMSは未指定時の長さ。合成・プレビュー・manifestが共有するcanonical値。
const (
	DefaultMoraDurationMS  = 140.0
	DefaultPauseDurationMS = 180.0
)

type Config struct {
	SpeechTiming    bool
	MoraDurationMS  float64
	PauseDurationMS float64
	// PauseContextはポーズ長の文脈化(B5)を有効にする。
	PauseContext bool
	// PauseContextStrengthはポーズ長補正の強度。0は既定1.0、負値は恒等。
	PauseContextStrength float64
	MoraDurationsMS      []float64
	// PhoneWeightsはモーラ内の音素時間比。nilなら既存の固定重みを使う。
	PhoneWeights       [][]float64
	PhoneWeightsSource string
	Predictions        []prosody.Prediction
	Tone               string
	Color              string
	AliasPolicy        voicebank.AliasPolicy
	// StretchAdaptは伸縮の音源適応(C3a)で全モーラのSpeechProfileを取得する。日本語のみ有効化する。
	StretchAdapt bool
}

type Plan struct {
	WordBoundaryEnvelope    bool                     `json:"word_boundary_envelope,omitempty"`
	SingleCV                bool                     `json:"single_cv,omitempty"`
	SpeechTiming            bool                     `json:"speech_timing,omitempty"`
	PhoneTimings            []PhoneTiming            `json:"phone_timings,omitempty"`
	PhoneTimingSource       string                   `json:"phone_timing_source,omitempty"`
	MissingPhones           []voicebank.SpeechGap    `json:"missing_phones,omitempty"`
	Version                 int                      `json:"version"`
	Voicebank               string                   `json:"voicebank"`
	Text                    string                   `json:"text,omitempty"`
	Reading                 string                   `json:"reading"`
	Language                string                   `json:"language,omitempty"`
	Phonemizer              string                   `json:"phonemizer,omitempty"`
	Tone                    string                   `json:"tone,omitempty"`
	Color                   string                   `json:"color,omitempty"`
	SelectionMode           string                   `json:"selection_mode"`
	AliasPolicy             string                   `json:"alias_policy"`
	RequestedAliasPolicy    string                   `json:"requested_alias_policy,omitempty"`
	JoinCostMode            string                   `json:"join_cost_mode"`
	JoinModelID             string                   `json:"join_model_id,omitempty"`
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

// Cloneはレンダラー用の独立したコピーを返す。診断結果を選択計画へ戻さない。
func Clone(source *Plan) *Plan {
	if source == nil {
		return nil
	}
	result := *source
	result.PhoneTimings = append([]PhoneTiming(nil), source.PhoneTimings...)
	result.MissingPhones = append([]voicebank.SpeechGap(nil), source.MissingPhones...)
	for i, gap := range source.MissingPhones {
		result.MissingPhones[i] = gap
		result.MissingPhones[i].Phones = append([]string(nil), gap.Phones...)
		result.MissingPhones[i].Aliases = append([]string(nil), gap.Aliases...)
	}
	result.Units = append([]Unit(nil), source.Units...)
	for index := range result.Units {
		result.Units[index].CodaPhones = append([]string(nil), source.Units[index].CodaPhones...)
		result.Units[index].CVTimingWarnings = append([]string(nil), source.Units[index].CVTimingWarnings...)
		if source.Units[index].SpeechProfile != nil {
			profile := *source.Units[index].SpeechProfile
			result.Units[index].SpeechProfile = &profile
		}
		result.Units[index].EntryValidation = append([]string(nil), source.Units[index].EntryValidation...)
		result.Units[index].CandidateRejections = append([]voicebank.CandidateRejection(nil), source.Units[index].CandidateRejections...)
	}
	result.BoundaryBridges = append([]BoundaryBridge(nil), source.BoundaryBridges...)
	result.BoundaryRepairDecisions = append([]BoundaryRepairDecision(nil), source.BoundaryRepairDecisions...)
	if source.Morae != nil {
		result.Morae = make([]frontend.Mora, len(source.Morae))
		for index, mora := range source.Morae {
			result.Morae[index] = cloneMora(mora)
		}
	}
	return &result
}

func cloneMora(mora frontend.Mora) frontend.Mora {
	mora.Phones = append([]frontend.Phone(nil), mora.Phones...)
	if mora.Aliases == nil {
		return mora
	}
	hints := *mora.Aliases
	if mora.Aliases.MainMissing != nil {
		hints.MainMissing = make(map[string][]string, len(mora.Aliases.MainMissing))
		for alias, phones := range mora.Aliases.MainMissing {
			hints.MainMissing[alias] = append([]string(nil), phones...)
		}
	}
	hints.EndingPhones = make([][]string, len(mora.Aliases.EndingPhones))
	for i, phones := range mora.Aliases.EndingPhones {
		hints.EndingPhones[i] = append([]string(nil), phones...)
	}
	hints.Main = append([]string(nil), mora.Aliases.Main...)
	hints.MainKinds = append([]string(nil), mora.Aliases.MainKinds...)
	hints.Transition = append([]string(nil), mora.Aliases.Transition...)
	hints.Endings = make([][]string, len(mora.Aliases.Endings))
	for index, endings := range mora.Aliases.Endings {
		hints.Endings[index] = append([]string(nil), endings...)
	}
	mora.Aliases = &hints
	return mora
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
	CodaPhones                  []string                       `json:"coda_phones,omitempty"`
	CodaFloorMS                 float64                        `json:"coda_floor_ms,omitempty"`
	CodaBoundaryLimited         bool                           `json:"coda_boundary_limited,omitempty"`
	CodaClosureMS               float64                        `json:"coda_closure_ms,omitempty"`
	CodaReleaseMS               float64                        `json:"coda_release_ms,omitempty"`
	CodaReleaseSeparated        bool                           `json:"coda_release_separated,omitempty"`
	WorldRenderMode             string                         `json:"world_render_mode,omitempty"`
	WorldRenderReason           string                         `json:"world_render_reason,omitempty"`
	WorldGapRepairEligible      bool                           `json:"world_gap_repair_eligible,omitempty"`
	WorldGapRepairReason        string                         `json:"world_gap_repair_reason,omitempty"`
	BoundaryEnvelope            string                         `json:"boundary_envelope,omitempty"`
	SpeechRetimeApplied         bool                           `json:"speech_retime_applied,omitempty"`
	SpeechJoinApplied           bool                           `json:"speech_join_applied,omitempty"`
	SpeechTransitionApplied     bool                           `json:"speech_transition_applied,omitempty"`
	StopBurstApplied            bool                           `json:"stop_burst_applied,omitempty"`
	StopBurstGain               float64                        `json:"stop_burst_gain,omitempty"`
	StopBurstReason             string                         `json:"stop_burst_reason,omitempty"`
	CVTimingApplied             bool                           `json:"cv_timing_applied,omitempty"`
	CVTimingWarnings            []string                       `json:"cv_timing_warnings,omitempty"`
	StretchAdapted              bool                           `json:"stretch_adapted,omitempty"`
	StretchLimitReason          string                         `json:"stretch_limit_reason,omitempty"`
	SpeechProfile               *voicebank.SpeechProfile       `json:"speech_profile,omitempty"`
	Position                    int                            `json:"position"`
	Role                        string                         `json:"role"`
	ParentPosition              int                            `json:"parent_position,omitempty"`
	TransitionFrom              string                         `json:"transition_from,omitempty"`
	TransitionTo                string                         `json:"transition_to,omitempty"`
	Mora                        string                         `json:"mora"`
	Alias                       string                         `json:"alias"`
	Source                      string                         `json:"source"`
	Silent                      bool                           `json:"silent,omitempty"`
	LongUnitGroup               int                            `json:"long_unit_group,omitempty"`
	LongUnitSize                int                            `json:"long_unit_size,omitempty"`
	OtoPath                     string                         `json:"oto_path"`
	OtoLine                     int                            `json:"oto_line"`
	NoteStartMS                 float64                        `json:"note_start_ms"`
	DurationMS                  float64                        `json:"duration_ms"`
	OffsetMS                    float64                        `json:"offset_ms"`
	ConsonantMS                 float64                        `json:"consonant_ms"`
	CutoffMS                    float64                        `json:"cutoff_ms"`
	PreutteranceMS              float64                        `json:"preutterance_ms"`
	OverlapMS                   float64                        `json:"overlap_ms"`
	PitchFactor                 float64                        `json:"pitch_factor"`
	EnergyFactor                float64                        `json:"energy_factor"`
	ResamplerVelocity           int                            `json:"resampler_velocity,omitempty"`
	ResamplerVolume             int                            `json:"resampler_volume,omitempty"`
	ResamplerFlags              string                         `json:"resampler_flags,omitempty"`
	ResamplerModulation         int                            `json:"resampler_modulation,omitempty"`
	ResamplerTempo              float64                        `json:"resampler_tempo,omitempty"`
	ResamplerVelocityOverride   bool                           `json:"-"`
	ResamplerVolumeOverride     bool                           `json:"-"`
	ResamplerFlagsOverride      bool                           `json:"-"`
	ResamplerModulationOverride bool                           `json:"-"`
	ResamplerTempoOverride      bool                           `json:"-"`
	TimingScale                 float64                        `json:"timing_scale"`
	EffectivePreutteranceMS     float64                        `json:"effective_preutterance_ms"`
	EffectiveConsonantMS        float64                        `json:"effective_consonant_ms"`
	EffectiveOverlapMS          float64                        `json:"effective_overlap_ms"`
	SourceF0Hz                  float64                        `json:"source_f0_hz,omitempty"`
	TargetF0Hz                  float64                        `json:"target_f0_hz,omitempty"`
	IntonationFactor            float64                        `json:"intonation_factor"`
	CandidateCount              int                            `json:"candidate_count"`
	TargetScore                 float64                        `json:"target_score"`
	JoinScore                   float64                        `json:"join_score"`
	TransitionJoinScore         float64                        `json:"transition_join_score,omitempty"`
	PathScore                   float64                        `json:"path_score"`
	AliasKind                   string                         `json:"alias_kind,omitempty"`
	FallbackTier                int                            `json:"fallback_tier"`
	SubbankID                   string                         `json:"subbank_id,omitempty"`
	Color                       string                         `json:"color,omitempty"`
	RequestedTone               string                         `json:"requested_tone,omitempty"`
	ResolvedTone                string                         `json:"resolved_tone,omitempty"`
	EntryStatus                 string                         `json:"entry_status,omitempty"`
	EntryValidation             []string                       `json:"entry_validation,omitempty"`
	CandidateRejections         []voicebank.CandidateRejection `json:"candidate_rejections,omitempty"`
}

func Build(bank *voicebank.Bank, reading string, morae []frontend.Mora, selections []voicebank.Selection, cfg Config) (*Plan, error) {
	if math.IsNaN(cfg.MoraDurationMS) || math.IsInf(cfg.MoraDurationMS, 0) {
		return nil, fmt.Errorf("mora duration must be finite, got %v", cfg.MoraDurationMS)
	}
	if math.IsNaN(cfg.PauseDurationMS) || math.IsInf(cfg.PauseDurationMS, 0) {
		return nil, fmt.Errorf("pause duration must be finite, got %v", cfg.PauseDurationMS)
	}
	if cfg.MoraDurationMS <= 0 {
		cfg.MoraDurationMS = DefaultMoraDurationMS
	}
	if cfg.PauseDurationMS <= 0 {
		cfg.PauseDurationMS = DefaultPauseDurationMS
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

	aliasPolicy := cfg.AliasPolicy
	if aliasPolicy == "" {
		aliasPolicy = voicebank.AliasPolicyAuto
	}
	result := &Plan{
		SpeechTiming: cfg.SpeechTiming,
		SingleCV:     voicebank.IsSingleCVSelections(selections),
		Version:      Version, Voicebank: bank.Root, Reading: reading,
		Morae: append([]frontend.Mora(nil), morae...),
		Tone:  cfg.Tone, Color: cfg.Color,
		SelectionMode: "viterbi", AliasPolicy: string(aliasPolicy), JoinCostMode: "handcrafted",
		PhoneTimingSource: cfg.PhoneWeightsSource,
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
				duration = cfg.PauseDurationMS * pauseContextFactor(morae, position, cfg)
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
		result.MissingPhones = append(result.MissingPhones, selection.MissingPhones...)
		duration, manuallySet := configuredMoraDuration(position, cfg)
		if !manuallySet {
			duration = durationFor(mora, cfg.MoraDurationMS)
			if prediction.DurationMS > 0 {
				duration = prediction.DurationMS
			} else if prediction.DurationFactor > 0 {
				duration *= prediction.DurationFactor
			}
		}
		phoneCursor := cursor
		phoneSpans, err := phoneSpansForMora(mora, duration, cfg.PhoneWeights, position)
		if err != nil {
			return nil, err
		}
		for i, phone := range mora.Phones {
			result.PhoneTimings = append(result.PhoneTimings, PhoneTiming{Position: position, Symbol: phone.Symbol, Role: phone.Role, StartMS: phoneCursor, DurationMS: phoneSpans[i]})
			phoneCursor += phoneSpans[i]
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
		// VCVの境界は発話タイミング補正なしでも解析し、伸縮だけ設定に従う。
		isVCV := aliasKind == voicebank.AliasVCV || voicebank.IsContextVCVAlias(selection.Alias)
		// C3aでは日本語の全モーラを対象にするため、必要な音源だけプロファイルを取る（キャッシュ前提）。
		needsProfile := cfg.SpeechTiming || result.SingleCV || isVCV || stopPhone(mora.Consonant) ||
			(cfg.StretchAdapt && japaneseSpeechMora(mora))
		if needsProfile && mora.Vowel != "" && mora.Vowel != "cl" && !mainUnit.Silent {
			profile := bank.CalibrateSpeech(selection.Entry)
			mainUnit.SpeechProfile = &profile
			if profile.Applied && !result.SingleCV && cfg.SpeechTiming {
				mainUnit.ConsonantMS = profile.SuggestedFixedMS
			}
		}
		if position < len(cfg.PhoneWeights) && cfg.PhoneWeights[position] != nil {
			applyPhoneTimingAnchor(&mainUnit, mora, phoneSpans)
		}
		mainUnit.AliasKind = string(aliasKind)
		mainUnit.TransitionJoinScore = selection.TransitionJoinScore
		result.Units = append(result.Units, mainUnit)
		if len(selection.Endings) > 0 {
			endingDuration := endingDurationFor(duration, len(selection.Endings))
			endingStart := cursor + duration - endingDuration*float64(len(selection.Endings))
			for index := range selection.Endings {
				start, span := endingStart+float64(index)*endingDuration, endingDuration
				codaFloorMS := 0.0
				if mora.Language == frontend.LanguageEnglish && mora.Aliases != nil && len(mora.Aliases.EndingPhones) > 0 {
					start, span, codaFloorMS = speechEndingTiming(mora, phoneSpans, selection.Endings[index].EndingIndex, cursor, duration)
				}
				endingUnit := unitFromSelection(&selection.Endings[index], position, start, span, prediction, "ending")
				endingIndex := selection.Endings[index].EndingIndex
				if mora.Language == frontend.LanguageEnglish && mora.Aliases != nil && endingIndex >= 0 && endingIndex < len(mora.Aliases.EndingPhones) {
					endingUnit.CodaPhones = append([]string(nil), mora.Aliases.EndingPhones[endingIndex]...)
				}
				if len(endingUnit.CodaPhones) > 0 {
					endingUnit.CodaFloorMS = codaFloorMS
				}
				if containsStopPhone(endingUnit.CodaPhones) && !endingUnit.Silent {
					profile := bank.CalibrateSpeech(selection.Endings[index].Entry)
					endingUnit.SpeechProfile = &profile
				}
				result.Units = append(result.Units, endingUnit)
			}
		}
		cursor += duration
	}
	result.DurationMS = cursor
	return result, nil
}

func containsStopPhone(phones []string) bool {
	for _, phone := range phones {
		if stopPhone(phone) {
			return true
		}
	}
	return false
}

func stopPhone(phone string) bool {
	switch strings.ToLower(strings.TrimSpace(phone)) {
	case "p", "b", "t", "d", "k", "g", "q", "cl", "py", "by", "ty", "dy", "ky", "gy":
		return true
	default:
		return false
	}
}

// japaneseSpeechMoraはかな入力（言語未指定）または日本語のモーラかを返す。
func japaneseSpeechMora(mora frontend.Mora) bool {
	return mora.Language == "" || mora.Language == frontend.LanguageJapanese
}

func phoneSpansForMora(mora frontend.Mora, duration float64, weights [][]float64, position int) ([]float64, error) {
	if position < 0 || position >= len(weights) || weights[position] == nil {
		return frontend.PhoneSpans(mora.Phones, duration), nil
	}
	if len(weights[position]) != len(mora.Phones) {
		return nil, fmt.Errorf("phone weights at position %d: got %d values for %d phones", position, len(weights[position]), len(mora.Phones))
	}
	sum := 0.0
	for index, weight := range weights[position] {
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
			return nil, fmt.Errorf("phone weight at position %d/%d is invalid: %v", position, index, weight)
		}
		sum += weight
	}
	if len(weights[position]) > 0 && (sum <= 0 || math.IsNaN(sum) || math.IsInf(sum, 0)) {
		return nil, fmt.Errorf("phone weights at position %d have no positive value", position)
	}
	spans := make([]float64, len(weights[position]))
	for index, weight := range weights[position] {
		spans[index] = duration * weight / sum
	}
	return spans, nil
}

func applyPhoneTimingAnchor(unit *Unit, mora frontend.Mora, spans []float64) {
	if unit == nil || len(spans) != len(mora.Phones) || unit.PreutteranceMS <= 0 {
		return
	}
	onsetMS := 0.0
	for index, phone := range mora.Phones {
		if phone.Role == "onset" {
			onsetMS += spans[index]
		}
	}
	if onsetMS <= 0 || math.IsNaN(onsetMS) || math.IsInf(onsetMS, 0) {
		return
	}
	unit.ConsonantMS = unit.PreutteranceMS + onsetMS
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
	case "n":
		return base * 0.9
	}
	if mora.Text == "ー" {
		return base * 1.2
	}
	return base
}
