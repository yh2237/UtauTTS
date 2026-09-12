package render

import (
	"math"
	"strings"

	"utautts/internal/connection"
	"utautts/internal/oto"
	"utautts/internal/plan"
)

const (
	minimumBoundaryBridgeMS        = 8
	maximumBoundaryBridgeMS        = 40
	boundaryBridgeSearchMS         = 5
	boundaryBridgeMeasureMarginMS  = 2
	boundaryBridgeMix              = 0.35
	minimumPeakImprovement         = 0.03
	maximumDeltaRMSRatio           = 1.02
	singleCVBoundaryMixVowel       = 0.22
	singleCVBoundaryMixSonorant    = 0.15
	singleCVBoundaryMixFricative   = 0.08
	singleCVBoundaryMinCorrelation = 0.42
	singleCVBoundaryMinImprovement = 0.005
	singleCVBoundaryDeltaRMSRatio  = 1.04
)

type renderedUnit struct {
	index        int
	unit         plan.Unit
	timing       effectiveTiming
	wave         []float64
	startFrame   int
	fadeInFrames int
}

type transitionMeasure struct {
	peak     float64
	deltaRMS float64
}

type boundaryRepairChoice struct {
	applied        bool
	candidateCount int
	startFrame     int
	endFrame       int
	lagFrames      int
	correlation    float64
	baseline       transitionMeasure
	selected       transitionMeasure
	wave           []float64
}

func applyBoundaryBridges(mix, mixWeights []float64, rendered []renderedUnit, synthesisPlan *plan.Plan, cfg Config, sampleRate int) {
	if (cfg.BoundaryBridgeMS <= 0 && !synthesisPlan.SpeechTiming && !synthesisPlan.SingleCV) || sampleRate <= 0 || len(rendered) < 2 {
		return
	}
	if synthesisPlan.SingleCV {
		applySingleCVBoundaryBridges(mix, mixWeights, rendered, synthesisPlan, sampleRate, cfg.BoundaryBridgeMS)
		return
	}

	bridgeMS := math.Max(minimumBoundaryBridgeMS, math.Min(maximumBoundaryBridgeMS, cfg.BoundaryBridgeMS))
	automaticSpeech := cfg.BoundaryBridgeMS <= 0
	if automaticSpeech {
		bridgeMS = 20
	}
	extractor := connection.NewExtractor()
	for index := 1; index < len(rendered); index++ {
		previous, current := rendered[index-1], rendered[index]
		if automaticSpeech && !speechVowelJoin(synthesisPlan, previous, current) {
			continue
		}
		if previous.index+1 != current.index || previous.unit.Role == "transition" || current.unit.Role == "transition" || previous.unit.Position+1 != current.unit.Position {
			continue
		}
		features := extractor.Pair(asOtoEntry(previous.unit), asOtoEntry(current.unit))
		if !features.PreviousOutgoing.Valid || !features.CurrentIncoming.Valid {
			continue
		}
		joinScore := connection.HandcraftedScore(features)
		if !automaticSpeech && joinScore > cfg.BoundaryBridgeThreshold {
			continue
		}

		choice := chooseBoundaryRepair(mix, mixWeights, previous, current, bridgeMS, sampleRate)
		if automaticSpeech && choice.correlation < 0.6 {
			choice.applied = false
			choice.selected = choice.baseline
		}
		selectedKind := "normal"
		if choice.applied {
			selectedKind = "phase-aligned-vowel-tail"
			for offset, value := range choice.wave {
				position := choice.startFrame + offset
				mix[position] = value
				mixWeights[position] = 1
			}
			synthesisPlan.BoundaryBridges = append(synthesisPlan.BoundaryBridges, plan.BoundaryBridge{
				UnitIndex:   current.index,
				Position:    current.unit.Position,
				StartMS:     framesToMS(choice.startFrame, sampleRate),
				EndMS:       framesToMS(choice.endFrame, sampleRate),
				DurationMS:  framesToMS(choice.endFrame-choice.startFrame, sampleRate),
				LagMS:       framesToMS(choice.lagFrames, sampleRate),
				JoinScore:   joinScore,
				Correlation: choice.correlation,
				Source:      previous.unit.Source,
				Kind:        selectedKind,
			})
		}
		synthesisPlan.BoundaryRepairDecisions = append(synthesisPlan.BoundaryRepairDecisions, plan.BoundaryRepairDecision{
			UnitIndex:        current.index,
			Position:         current.unit.Position,
			CandidateCount:   choice.candidateCount,
			SelectedKind:     selectedKind,
			Applied:          choice.applied,
			DurationMS:       framesToMS(choice.endFrame-choice.startFrame, sampleRate),
			LagMS:            framesToMS(choice.lagFrames, sampleRate),
			JoinScore:        joinScore,
			Correlation:      choice.correlation,
			BaselinePeak:     choice.baseline.peak,
			SelectedPeak:     choice.selected.peak,
			BaselineDeltaRMS: choice.baseline.deltaRMS,
			SelectedDeltaRMS: choice.selected.deltaRMS,
		})
	}
}

// applySingleCVBoundaryBridges はCV境界へ短い母音末尾だけを補う
// 次の子音を主信号として残し子音の欠落を防ぐ
func applySingleCVBoundaryBridges(mix, mixWeights []float64, rendered []renderedUnit, synthesisPlan *plan.Plan, sampleRate int, maximumMS float64) {
	if sampleRate <= 0 || len(mix) == 0 || len(mixWeights) != len(mix) {
		return
	}
	for index := 1; index < len(rendered); index++ {
		previous, current := rendered[index-1], rendered[index]
		if !singleCVBoundaryEligible(synthesisPlan, previous, current) {
			continue
		}
		if singleCVProtectedOnset(synthesisPlan, current.unit) {
			// 破裂音と破擦音の閉鎖や破裂を残す
			continue
		}
		profile := singleCVBoundaryProfileForOnset(singleCVOnset(synthesisPlan, current.unit))
		if maximumMS > 0 {
			profile.widthMS = math.Min(profile.widthMS, maximumMS)
		}
		start, end := singleCVBoundaryWindow(current, profile.widthMS, len(mix), sampleRate)
		widthFrames := end - start
		if widthFrames < msToFrames(minimumBoundaryBridgeMS, sampleRate) {
			continue
		}
		target := normalizedMix(mix, mixWeights, start, end)
		segment, lagFrames, correlation := bestAlignedVowelSegment(previous, target, widthFrames, sampleRate)
		if len(segment) != widthFrames || correlation < profile.minCorrelation {
			continue
		}
		segment = matchLevelAndMean(segment, target)
		margin := max(1, msToFrames(boundaryBridgeMeasureMarginMS, sampleRate))
		measureStart := max(1, start-margin)
		measureEnd := min(len(mix), end+margin)
		baselineWindow := normalizedMix(mix, mixWeights, measureStart, measureEnd)
		baseline := measureTransition(baselineWindow)
		if baseline.peak <= 1e-9 || baseline.deltaRMS <= 1e-9 {
			continue
		}
		candidateWindow := append([]float64(nil), baselineWindow...)
		windowOffset := start - measureStart
		mixAmount := profile.maxMix * singleCVBoundaryCorrelationGain(correlation, profile.minCorrelation)
		for frame := range target {
			alpha := mixAmount * bridgeEnvelope(frame, widthFrames)
			candidateWindow[windowOffset+frame] = target[frame]*(1-alpha) + segment[frame]*alpha
		}
		selected := measureTransition(candidateWindow)
		if !singleCVBoundaryImproves(baseline, selected) {
			continue
		}
		for frame := range target {
			mix[start+frame] = candidateWindow[windowOffset+frame]
			mixWeights[start+frame] = 1
		}
		synthesisPlan.Units[current.index].SpeechJoinApplied = true
		synthesisPlan.BoundaryBridges = append(synthesisPlan.BoundaryBridges, plan.BoundaryBridge{
			UnitIndex: current.index, Position: current.unit.Position,
			StartMS: framesToMS(start, sampleRate), EndMS: framesToMS(end, sampleRate),
			DurationMS: framesToMS(widthFrames, sampleRate), LagMS: framesToMS(lagFrames, sampleRate),
			Correlation: correlation, Source: previous.unit.Source, Kind: "single-cv-vowel-tail",
		})
		synthesisPlan.BoundaryRepairDecisions = append(synthesisPlan.BoundaryRepairDecisions, plan.BoundaryRepairDecision{
			UnitIndex: current.index, Position: current.unit.Position, CandidateCount: 1,
			SelectedKind: "single-cv-vowel-tail", Applied: true, DurationMS: framesToMS(widthFrames, sampleRate),
			LagMS: framesToMS(lagFrames, sampleRate), Correlation: correlation,
			BaselinePeak: baseline.peak, SelectedPeak: selected.peak,
			BaselineDeltaRMS: baseline.deltaRMS, SelectedDeltaRMS: selected.deltaRMS,
		})
	}
}

type singleCVBoundaryProfile struct {
	widthMS        float64
	maxMix         float64
	minCorrelation float64
}

func singleCVBoundaryProfileForOnset(onset string) singleCVBoundaryProfile {
	switch strings.ToLower(strings.TrimSpace(onset)) {
	case "s", "sh", "f", "h", "z", "zh", "x", "v":
		return singleCVBoundaryProfile{widthMS: 8, maxMix: singleCVBoundaryMixFricative, minCorrelation: 0.48}
	case "m", "n", "ny", "r", "l", "w", "y", "my", "ry", "ly":
		return singleCVBoundaryProfile{widthMS: 12, maxMix: singleCVBoundaryMixSonorant, minCorrelation: singleCVBoundaryMinCorrelation}
	default:
		return singleCVBoundaryProfile{widthMS: 16, maxMix: singleCVBoundaryMixVowel, minCorrelation: singleCVBoundaryMinCorrelation}
	}
}

func singleCVBoundaryWindow(current renderedUnit, widthMS float64, mixLength, sampleRate int) (int, int) {
	if sampleRate <= 0 || mixLength <= 0 || current.fadeInFrames <= 0 {
		return 0, 0
	}
	handoffStart := max(1, current.startFrame)
	handoffEnd := min(mixLength, current.startFrame+current.fadeInFrames)
	if handoffEnd <= handoffStart {
		return 0, 0
	}
	width := min(msToFrames(widthMS, sampleRate), handoffEnd-handoffStart)
	if width <= 0 {
		return 0, 0
	}
	// 手渡し区間の後半だけを使い次の子音の立ち上がりを保護する
	return handoffEnd - width, handoffEnd
}

func singleCVBoundaryCorrelationGain(correlation, minimum float64) float64 {
	if correlation <= minimum {
		return 0
	}
	progress := (correlation - minimum) / math.Max(1e-9, 1-minimum)
	return 0.7 + 0.3*math.Max(0, math.Min(1, progress))
}

func singleCVBoundaryImproves(baseline, selected transitionMeasure) bool {
	if baseline.peak <= 1e-9 || baseline.deltaRMS <= 1e-9 {
		return false
	}
	if selected.peak > baseline.peak || selected.deltaRMS > baseline.deltaRMS*singleCVBoundaryDeltaRMSRatio {
		return false
	}
	peakImprovement := (baseline.peak - selected.peak) / baseline.peak
	deltaImprovement := (baseline.deltaRMS - selected.deltaRMS) / baseline.deltaRMS
	return peakImprovement >= singleCVBoundaryMinImprovement ||
		deltaImprovement >= singleCVBoundaryMinImprovement
}

func chooseBoundaryRepair(mix, mixWeights []float64, previous, current renderedUnit, maximumMS float64, sampleRate int) boundaryRepairChoice {
	choice := boundaryRepairChoice{candidateCount: 1}
	if sampleRate <= 0 || len(mix) == 0 || len(mixWeights) != len(mix) {
		return choice
	}
	handoffStart := max(1, current.startFrame)
	handoffEnd := min(len(mix), current.startFrame+current.fadeInFrames)
	if handoffEnd-handoffStart < msToFrames(minimumBoundaryBridgeMS, sampleRate) {
		return choice
	}
	margin := msToFrames(boundaryBridgeMeasureMarginMS, sampleRate)
	measureStart := max(1, handoffStart-margin)
	measureEnd := min(len(mix), handoffEnd+margin)
	baselineWindow := normalizedMix(mix, mixWeights, measureStart, measureEnd)
	choice.baseline = measureTransition(baselineWindow)
	choice.selected = choice.baseline
	if choice.baseline.peak <= 1e-9 || choice.baseline.deltaRMS <= 1e-9 {
		return choice
	}

	peakFrame := measureStart + peakDerivativeIndex(baselineWindow)
	bestScore := 0.0
	for _, widthMS := range boundaryBridgeWidths(maximumMS) {
		widthFrames := msToFrames(widthMS, sampleRate)
		if widthFrames < 2 || widthFrames > handoffEnd-handoffStart {
			continue
		}
		for _, start := range boundaryCandidateStarts(handoffStart, handoffEnd, widthFrames, peakFrame) {
			end := start + widthFrames
			target := normalizedMix(mix, mixWeights, start, end)
			segment, lagFrames, correlation := bestAlignedVowelSegment(previous, target, widthFrames, sampleRate)
			if len(segment) != widthFrames || correlation <= 0 {
				continue
			}
			choice.candidateCount++
			segment = matchLevelAndMean(segment, target)
			candidateWindow := append([]float64(nil), baselineWindow...)
			windowOffset := start - measureStart
			for frame := 0; frame < widthFrames; frame++ {
				alpha := boundaryBridgeMix * bridgeEnvelope(frame, widthFrames)
				candidateWindow[windowOffset+frame] = target[frame]*(1-alpha) + segment[frame]*alpha
			}
			measure := measureTransition(candidateWindow)
			peakImprovement := (choice.baseline.peak - measure.peak) / choice.baseline.peak
			deltaImprovement := (choice.baseline.deltaRMS - measure.deltaRMS) / choice.baseline.deltaRMS
			if peakImprovement < minimumPeakImprovement || measure.deltaRMS > choice.baseline.deltaRMS*maximumDeltaRMSRatio {
				continue
			}
			score := peakImprovement + 0.35*deltaImprovement + 0.03*correlation - 0.01*widthMS/maximumMS
			if score <= bestScore {
				continue
			}
			bestScore = score
			choice.applied = true
			choice.startFrame = start
			choice.endFrame = end
			choice.lagFrames = lagFrames
			choice.correlation = correlation
			choice.selected = measure
			choice.wave = append([]float64(nil), candidateWindow[windowOffset:windowOffset+widthFrames]...)
		}
	}
	return choice
}

func boundaryCandidateStarts(handoffStart, handoffEnd, width, peakFrame int) []int {
	latest := handoffEnd - width
	if width <= 0 || latest < handoffStart {
		return nil
	}
	values := []int{
		handoffStart,
		latest,
		max(handoffStart, min(latest, peakFrame-width/2)),
		max(handoffStart, min(latest, peakFrame-width)),
		max(handoffStart, min(latest, peakFrame)),
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		duplicate := false
		for _, existing := range result {
			if existing == value {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, value)
		}
	}
	return result
}

func boundaryBridgeWidths(maximumMS float64) []float64 {
	maximumMS = math.Max(minimumBoundaryBridgeMS, math.Min(maximumBoundaryBridgeMS, maximumMS))
	base := []float64{8, 12, 20, maximumMS}
	result := make([]float64, 0, len(base))
	for _, width := range base {
		if width > maximumMS+1e-9 {
			continue
		}
		duplicate := false
		for _, existing := range result {
			if math.Abs(existing-width) < 1e-9 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, width)
		}
	}
	return result
}

func bestAlignedVowelSegment(unit renderedUnit, target []float64, frames, sampleRate int) ([]float64, int, float64) {
	if frames <= 1 || len(target) != frames || len(unit.wave) < frames {
		return nil, 0, 0
	}
	vowelStart := msToFrames(unit.timing.consonantMS, sampleRate)
	vowelEnd := msToFrames(unit.timing.preutteranceMS+unit.unit.DurationMS, sampleRate)
	vowelStart = max(0, min(vowelStart, len(unit.wave)))
	vowelEnd = max(vowelStart, min(vowelEnd, len(unit.wave)))
	nominalStart := vowelEnd - frames
	searchFrames := msToFrames(boundaryBridgeSearchMS, sampleRate)
	low := max(vowelStart, nominalStart-searchFrames)
	high := min(len(unit.wave)-frames, nominalStart+searchFrames)
	if low > high {
		return nil, 0, 0
	}
	step := max(1, sampleRate/16000)
	bestStart := low
	bestCorrelation := math.Inf(-1)
	for start := low; start <= high; start += step {
		correlation := normalizedCorrelation(unit.wave[start:start+frames], target)
		if correlation > bestCorrelation {
			bestStart = start
			bestCorrelation = correlation
		}
	}
	if correlation := normalizedCorrelation(unit.wave[high:high+frames], target); correlation > bestCorrelation {
		bestStart = high
		bestCorrelation = correlation
	}
	return append([]float64(nil), unit.wave[bestStart:bestStart+frames]...), bestStart - nominalStart, bestCorrelation
}

func normalizedMix(mix, weights []float64, start, end int) []float64 {
	start = max(0, min(start, len(mix)))
	end = max(start, min(end, len(mix)))
	result := make([]float64, end-start)
	for index := range result {
		position := start + index
		result[index] = mix[position]
		if weights[position] > 1e-12 {
			result[index] /= weights[position]
		}
	}
	return result
}

func matchLevelAndMean(source, target []float64) []float64 {
	if len(source) == 0 || len(source) != len(target) {
		return append([]float64(nil), source...)
	}
	sourceMean, targetMean := meanValue(source), meanValue(target)
	sourceRMS, targetRMS := centeredRMS(source, sourceMean), centeredRMS(target, targetMean)
	scale := 1.0
	if sourceRMS > 1e-9 && targetRMS > 1e-9 {
		scale = math.Max(0.75, math.Min(1.25, targetRMS/sourceRMS))
	}
	result := make([]float64, len(source))
	for index, value := range source {
		result[index] = (value-sourceMean)*scale + targetMean
	}
	return result
}

func normalizedCorrelation(left, right []float64) float64 {
	if len(left) < 2 || len(left) != len(right) {
		return 0
	}
	leftMean, rightMean := meanValue(left), meanValue(right)
	cross, leftEnergy, rightEnergy := 0.0, 0.0, 0.0
	for index := range left {
		a := left[index] - leftMean
		b := right[index] - rightMean
		cross += a * b
		leftEnergy += a * a
		rightEnergy += b * b
	}
	if leftEnergy <= 1e-12 || rightEnergy <= 1e-12 {
		return 0
	}
	return cross / math.Sqrt(leftEnergy*rightEnergy)
}

func measureTransition(wave []float64) transitionMeasure {
	if len(wave) < 2 {
		return transitionMeasure{}
	}
	peak, energy := 0.0, 0.0
	for index := 1; index < len(wave); index++ {
		delta := wave[index] - wave[index-1]
		peak = math.Max(peak, math.Abs(delta))
		energy += delta * delta
	}
	return transitionMeasure{peak: peak, deltaRMS: math.Sqrt(energy / float64(len(wave)-1))}
}

func peakDerivativeIndex(wave []float64) int {
	best, peak := 0, 0.0
	for index := 1; index < len(wave); index++ {
		if delta := math.Abs(wave[index] - wave[index-1]); delta > peak {
			best, peak = index, delta
		}
	}
	return best
}

func meanValue(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func centeredRMS(values []float64, mean float64) float64 {
	if len(values) == 0 {
		return 0
	}
	energy := 0.0
	for _, value := range values {
		delta := value - mean
		energy += delta * delta
	}
	return math.Sqrt(energy / float64(len(values)))
}

func asOtoEntry(unit plan.Unit) oto.Entry {
	return oto.Entry{
		Filename:     unit.Source,
		Alias:        unit.Alias,
		Offset:       unit.OffsetMS,
		Fixed:        unit.ConsonantMS,
		Blank:        unit.CutoffMS,
		Preutterance: unit.PreutteranceMS,
		Overlap:      unit.OverlapMS,
		OtoPath:      unit.OtoPath,
		Line:         unit.OtoLine,
	}
}

func bridgeEnvelope(frame, total int) float64 {
	if total <= 1 || frame < 0 || frame >= total {
		return 0
	}
	progress := float64(frame) / float64(total-1)
	return math.Min(1, 4*smoothstep(progress)*smoothstep(1-progress))
}

func framesToMS(frames, sampleRate int) float64 {
	if sampleRate <= 0 {
		return 0
	}
	return float64(frames) * 1000 / float64(sampleRate)
}
