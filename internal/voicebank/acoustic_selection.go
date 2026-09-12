package voicebank

import (
	"math"
	"sort"
	"strings"

	"utautts/internal/acoustic"
	"utautts/internal/connection"
	"utautts/internal/oto"
)

const (
	AcousticModeDryRun = "dry-run"
	AcousticModeApply  = "apply"
)

func validAcousticMode(mode string) bool {
	return mode == "" || mode == AcousticModeDryRun || mode == AcousticModeApply
}

// 音響スコアは既存の選択を支配しない小さな補正値に留める。
func (b *Bank) populateAcousticScores(candidates []Selection, previous []Selection, mode string) {
	if len(candidates) == 0 {
		return
	}
	if b.extractor == nil {
		b.extractor = connection.NewExtractor()
	}
	frames := make(map[oto.Entry]acoustic.Frame)
	groups := make(map[string][]acoustic.Frame)
	for _, candidate := range candidates {
		frame := b.entryAcousticFrame(candidate.Entry, candidate.Kind, frames)
		if frame.Valid {
			groups[acousticGroupKey(b.Root, candidate)] = append(groups[acousticGroupKey(b.Root, candidate)], frame)
		}
		if candidate.Transition != nil {
			b.entryAcousticFrame(candidate.Transition.Entry, candidate.Transition.Kind, frames)
		}
	}
	medians := make(map[string]acoustic.Frame, len(groups))
	for key, values := range groups {
		medians[key] = acousticMedianFrame(values)
	}

	for index := range candidates {
		candidate := &candidates[index]
		frame := frames[candidate.Entry]
		if frame.Valid {
			candidate.AcousticTargetScore = acousticTargetAdjustment(frame, medians[acousticGroupKey(b.Root, *candidate)])
		}
		if len(previous) > 0 {
			best := 0.0
			for _, prior := range previous {
				adjustment := acousticPairAdjustment(b.extractor.Pair(currentStartEntry(prior), currentStartEntry(*candidate)))
				if adjustment > best {
					best = adjustment
				}
			}
			candidate.AcousticJoinScore = best
		}
		if candidate.Transition != nil {
			candidate.Transition.AcousticJoinScore = acousticPairAdjustment(
				b.extractor.Pair(candidate.Transition.Entry, candidate.Entry),
			)
		}
	}

	// marginは閾値ではなく、局所候補の1位と2位の差を示す診断値。
	best, second := math.Inf(-1), math.Inf(-1)
	for _, candidate := range candidates {
		score := candidate.TargetScore + candidate.PreferenceScore
		if mode == AcousticModeApply {
			score += candidate.AcousticTargetScore
		}
		if score > best {
			second, best = best, score
		} else if score > second {
			second = score
		}
	}
	margin := 0.0
	if second > math.Inf(-1) {
		margin = best - second
	}
	for index := range candidates {
		candidates[index].SelectionMargin = margin
	}
}

func (b *Bank) entryAcousticFrame(entry oto.Entry, kind AliasKind, cache map[oto.Entry]acoustic.Frame) acoustic.Frame {
	if frame, ok := cache[entry]; ok {
		return frame
	}
	boundary := b.extractor.Boundary(entry)
	frame := boundary.Incoming
	if kind == AliasVCV || IsContextVCVAlias(entry.Alias) {
		// VCVはfixedの終端側を母音の代表点として候補を比較する。
		// incoming側は閉鎖区間になりやすく録音間比較に向かない。
		frame = boundary.Outgoing
	}
	if !frame.Valid {
		frame = boundary.Outgoing
	}
	cache[entry] = frame
	return frame
}

func acousticGroupKey(root string, candidate Selection) string {
	group := candidate.Entry.SourceGroup
	if group == "" {
		group = sourceGroup(root, candidate.Entry)
	}
	return strings.Join([]string{candidate.Alias, candidate.SubbankID, candidate.Color, group}, "\x00")
}

func acousticMedianFrame(values []acoustic.Frame) acoustic.Frame {
	result := acoustic.Frame{Valid: len(values) > 0}
	if len(values) == 0 {
		return result
	}
	rms := make([]float64, 0, len(values))
	f0 := make([]float64, 0, len(values))
	for _, value := range values {
		rms = append(rms, value.RMSDB)
		if value.F0Hz > 0 {
			f0 = append(f0, value.F0Hz)
		}
	}
	result.RMSDB = medianFloat64(rms)
	result.F0Hz = medianFloat64(f0)
	return result
}

func medianFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	middle := len(copyValues) / 2
	if len(copyValues)%2 == 1 {
		return copyValues[middle]
	}
	return (copyValues[middle-1] + copyValues[middle]) / 2
}

func acousticTargetAdjustment(frame, median acoustic.Frame) float64 {
	if !frame.Valid || !median.Valid {
		return 0
	}
	result := -math.Min(3, math.Abs(frame.RMSDB-median.RMSDB)*0.08)
	if frame.F0Hz > 0 && median.F0Hz > 0 {
		cents := math.Abs(1200 * math.Log2(frame.F0Hz/median.F0Hz))
		result -= math.Min(3, cents*0.004)
	}
	return result
}

func acousticPairAdjustment(features connection.PairFeatures) float64 {
	if !features.PreviousOutgoing.Valid || !features.CurrentIncoming.Valid {
		return 0
	}
	spectrumWeight, rmsWeight := 0.08, 0.08
	if features.CurrentVCV {
		spectrumWeight, rmsWeight = 0.045, 0.045
	}
	result := -math.Min(2, features.SpectrumDelta*spectrumWeight)
	result -= math.Min(1.5, features.RMSDelta*rmsWeight)
	if features.PreviousOutgoing.F0Hz > 0 && features.CurrentIncoming.F0Hz > 0 {
		result -= math.Min(2, features.F0DeltaCents*0.004)
	} else if features.VoicingMismatch && !features.CurrentVCV {
		result -= 0.75
	}
	if features.WaveformCorrelation > 0 {
		result += math.Min(0.75, features.WaveformCorrelation*0.25)
	}
	return result
}
