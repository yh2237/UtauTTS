package main

import (
	"math"
	"strings"

	"utautts/internal/jsut"
	"utautts/internal/provider"
)

type worldSpeechMap struct {
	coda                                                                                bool
	sourceOnset, targetOnset, sourceFixed, targetFixed, sourceEnd, targetEnd, protected float64
}

func worldSpeechAnchors(item unit, duration float64) (worldSpeechMap, bool) {
	if item.Speech == nil || item.Speech.PreserveStopOnly {
		return worldSpeechMap{}, false
	}
	// Analysis starts at the preceding WORLD frame rather than exactly oto.offset.
	shift := math.Max(0, item.OffsetMS) - math.Floor(math.Max(0, item.OffsetMS)/worldFramePeriodMS)*worldFramePeriodMS
	a := worldSpeechMap{sourceOnset: item.Speech.SourceOnsetMS + shift, targetOnset: item.Speech.TargetOnsetMS,
		sourceFixed: item.ConsonantMS + shift, sourceEnd: duration, targetEnd: item.RequiredLengthMS}
	for _, v := range []float64{a.sourceOnset, a.targetOnset, a.sourceEnd, item.OffsetMS, item.Speech.TargetFixedMS} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return worldSpeechMap{}, false
		}
	}
	if item.Speech.CodaRelease {
		a.targetEnd = item.SkipMS + item.LengthMS
		if !math.IsNaN(a.targetEnd) && !math.IsInf(a.targetEnd, 0) && a.sourceOnset >= 0 && a.targetOnset >= 0 && (a.targetOnset > 0 || a.sourceOnset == 0) && a.sourceEnd-a.sourceOnset >= 10 && a.targetEnd-a.targetOnset >= 10 {
			a.coda = true
			a.targetFixed = a.targetEnd
			if item.Speech.ProtectStop {
				a.protected = math.Min(30, math.Min(a.sourceEnd-a.sourceOnset-4, a.targetEnd-a.targetOnset-4))
			}
			return a, true
		}
		return worldSpeechMap{}, false
	}
	for _, v := range []float64{a.sourceOnset, a.targetOnset, a.sourceFixed, a.sourceEnd, a.targetEnd, item.OffsetMS} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return worldSpeechMap{}, false
		}
	}
	if a.sourceOnset < 4 || a.targetOnset < 4 || a.sourceFixed-a.sourceOnset < 4 || a.sourceEnd-a.sourceFixed < 20 || a.targetEnd-a.targetOnset < 24 {
		return worldSpeechMap{}, false
	}
	if item.Speech.TargetFixedMS > 0 {
		a.targetFixed = math.Min(a.targetEnd-20, math.Max(a.targetOnset+4, item.Speech.TargetFixedMS))
	} else {
		ratio := (a.targetEnd - a.targetOnset) / (a.sourceEnd - a.sourceOnset)
		transition := (a.sourceFixed - a.sourceOnset) * math.Max(.75, math.Min(1.25, math.Sqrt(ratio)))
		a.targetFixed = math.Min(a.targetEnd-20, a.targetOnset+math.Max(4, transition))
	}
	if item.Speech.ProtectStop {
		a.protected = math.Min(8, math.Min(a.sourceOnset-4, a.targetOnset-4))
	}
	return a, true
}

func (a worldSpeechMap) sourceTime(t float64) float64 {
	t = math.Max(0, math.Min(a.targetEnd, t))
	if a.coda {
		if t < a.targetOnset {
			return t * a.sourceOnset / a.targetOnset
		}
		if t < a.targetOnset+a.protected {
			return a.sourceOnset + t - a.targetOnset
		}
		return a.sourceOnset + a.protected + (t-a.targetOnset-a.protected)*(a.sourceEnd-a.sourceOnset-a.protected)/(a.targetEnd-a.targetOnset-a.protected)
	}
	switch {
	case t < a.targetOnset-a.protected:
		return t * (a.sourceOnset - a.protected) / (a.targetOnset - a.protected)
	case t < a.targetOnset:
		return a.sourceOnset + t - a.targetOnset
	case t < a.targetFixed:
		return a.sourceOnset + (t-a.targetOnset)*(a.sourceFixed-a.sourceOnset)/(a.targetFixed-a.targetOnset)
	default:
		return a.sourceFixed + (t-a.targetFixed)*(a.sourceEnd-a.sourceFixed)/(a.targetEnd-a.targetFixed)
	}
}

// Smooth only a short, fully voiced repeated-vowel boundary in the mixed
// features. Cached source features and the target F0 curve remain untouched.
func applyWorldSpeechJoins(input manifest, features *worldFeatures) map[int]provider.WorldSpeechResult {
	report := make(map[int]provider.WorldSpeechResult)
	lastEnd := -1
	for index, item := range input.Units {
		if item.Speech == nil || !item.Speech.VowelJoin || index == 0 {
			continue
		}
		joinMS := item.Speech.TargetJoinMS
		if joinMS <= 0 {
			joinMS = item.Speech.TargetOnsetMS
		}
		centerMS := item.PositionMS + joinMS - item.SkipMS
		if math.IsNaN(centerMS) || math.IsInf(centerMS, 0) {
			continue
		}
		center := int(math.Round(centerMS / worldFramePeriodMS))
		start, end := center-2, center+2
		if start <= lastEnd || start < 0 || end >= features.Frames {
			continue
		}
		leftMS, rightMS := float64(start)*worldFramePeriodMS, float64(end)*worldFramePeriodMS
		previous := input.Units[index-1]
		if previous.PositionMS+previous.LengthMS < leftMS || item.PositionMS > rightMS {
			continue
		}
		blocked := false
		for j, other := range input.Units {
			if j != index && j != index-1 && other.PositionMS < rightMS && other.PositionMS+other.LengthMS > leftMS {
				blocked = true
				break
			}
		}
		if blocked || !smoothWorldVowel(features, start, end) {
			continue
		}
		report[item.Speech.UnitIndex] = provider.WorldSpeechResult{UnitIndex: item.Speech.UnitIndex, JoinApplied: true}
		lastEnd = end
	}
	return report
}

func smoothWorldVowel(f *worldFeatures, start, end int) bool {
	bins := f.FFTSize/2 + 1
	for frame := start; frame <= end; frame++ {
		if f.F0[frame] <= 71 {
			return false
		}
	}
	logs := make([]float64, (end-start+1)*bins)
	for i := range logs {
		v := f.Spectrum[start*bins+i]
		if v <= 1e-12 || math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
		logs[i] = math.Log(v)
	}
	candidate := append([]float64(nil), logs...)
	changeEnergy := 0.0
	for frame := 1; frame < end-start; frame++ {
		for bin := 0; bin < bins; bin++ {
			i := frame*bins + bin
			change := .2 * (logs[i-bins] + logs[i+bins] - 2*logs[i])
			// Limit local spectral power changes to about 1.5 dB per bin.
			changeEnergy += change * change
			candidate[i] += math.Max(-.35, math.Min(.35, change))
		}
	}
	if changeEnergy/float64(len(logs)-2*bins) > .35*.35 {
		return false
	}
	curvature := func(values []float64) float64 {
		sum := 0.0
		for i := bins; i < len(values)-bins; i++ {
			v := values[i-bins] + values[i+bins] - 2*values[i]
			sum += v * v
		}
		return sum / float64(len(values)-2*bins)
	}
	before, after := curvature(logs), curvature(candidate)
	if before < .0009 || after > before*.9 {
		return false
	}
	ap := append([]float64(nil), f.Aperiodicity[start*bins:(end+1)*bins]...)
	for frame := 1; frame < end-start; frame++ {
		for bin := 0; bin < bins; bin++ {
			i := frame*bins + bin
			f.Spectrum[start*bins+i] = math.Exp(candidate[i])
			f.Aperiodicity[start*bins+i] = math.Max(0, math.Min(1, .6*ap[i]+.2*(ap[i-bins]+ap[i+bins])))
		}
	}
	return true
}

func applyWorldTransitionModel(input manifest, features *worldFeatures, report map[int]provider.WorldSpeechResult) error {
	if strings.TrimSpace(input.TransitionModelPath) == "" || input.TransitionStrength <= 0 {
		return nil
	}
	strength := math.Min(.35, input.TransitionStrength)
	model, err := jsut.LoadTransitionTCN(input.TransitionModelPath)
	if err != nil {
		return err
	}
	for _, item := range input.Units {
		if item.Speech == nil || item.Speech.TransitionLeftPhone == "" || item.Speech.TransitionRightPhone == "" {
			continue
		}
		center := int(math.Round((item.PositionMS + item.Speech.TargetJoinMS - item.SkipMS) / worldFramePeriodMS))
		half := model.PositionBins / 2
		start, end := center-half, center+(model.PositionBins-half-1)
		if start < 0 || end >= features.Frames {
			continue
		}
		left := worldTransitionFrame(features, start, input.SampleRate)
		right := worldTransitionFrame(features, end, input.SampleRate)
		predictions, ok := model.Predict(item.Speech.TransitionLeftPhone, item.Speech.TransitionRightPhone, left, right)
		if !ok || len(predictions) != model.PositionBins || !transitionPredictionSafe(predictions) {
			continue
		}
		bins := features.FFTSize/2 + 1
		for position, prediction := range predictions {
			frame := start + position
			envelope := math.Sin(math.Pi * float64(position) / float64(model.PositionBins-1))
			rms := math.Max(-4, math.Min(4, prediction.RMSResidualDB))
			for bin := 0; bin < bins; bin++ {
				frequency := float64(bin) * float64(input.SampleRate) / float64(features.FFTSize)
				shape := transitionBandValue(prediction.SpectrumResidualDB, frequency, input.SampleRate)
				shape = math.Max(-6, math.Min(6, shape))
				features.Spectrum[frame*bins+bin] *= math.Pow(10, strength*envelope*(rms+shape)/10)
			}
		}
		entry := report[item.Speech.UnitIndex]
		entry.UnitIndex = item.Speech.UnitIndex
		entry.TransitionApplied = true
		report[item.Speech.UnitIndex] = entry
	}
	return nil
}

// transitionPredictionSafe は数値異常の推定を除外する。
func transitionPredictionSafe(predictions []jsut.TransitionPrediction) bool {
	if len(predictions) < 3 {
		return false
	}
	for _, prediction := range predictions {
		if math.IsNaN(prediction.RMSResidualDB) || math.IsInf(prediction.RMSResidualDB, 0) {
			return false
		}
		for _, value := range prediction.SpectrumResidualDB {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return false
			}
		}
	}
	return true
}

func worldTransitionFrame(features *worldFeatures, frame, sampleRate int) jsut.Frame {
	bins := features.FFTSize/2 + 1
	values := make([]float64, 10)
	mean := 0.0
	for band := range values {
		frequency := 100 * math.Pow(math.Min(8000, float64(sampleRate)*.45)/100, float64(band)/9)
		bin := max(0, min(bins-1, int(math.Round(frequency*float64(features.FFTSize)/float64(sampleRate)))))
		values[band] = 10 * math.Log10(math.Max(1e-12, features.Spectrum[frame*bins+bin]))
		mean += values[band]
	}
	mean /= float64(len(values))
	for band := range values {
		values[band] -= mean
	}
	energy := 0.0
	for bin := 0; bin < bins; bin++ {
		energy += features.Spectrum[frame*bins+bin]
	}
	return jsut.Frame{Valid: true, RMSDB: 10 * math.Log10(math.Max(1e-12, energy/float64(bins))), F0Hz: features.F0[frame], SpectrumDB: values}
}

func transitionBandValue(values []float64, frequency float64, sampleRate int) float64 {
	if len(values) == 0 {
		return 0
	}
	minimum, maximum := 100.0, math.Min(8000, float64(sampleRate)*.45)
	if frequency <= minimum {
		return values[0]
	}
	if frequency >= maximum {
		return values[len(values)-1]
	}
	position := math.Log(frequency/minimum) / math.Log(maximum/minimum) * float64(len(values)-1)
	left := int(math.Floor(position))
	ratio := position - float64(left)
	return values[left] + ratio*(values[left+1]-values[left])
}
