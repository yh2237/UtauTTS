package main

import "math"

// 音源波形と破裂音補強用の高域成分を保持する。
type protectedStopSource struct {
	sampleRate int
	samples    []float64
	transient  []float64
}

// WORLDのフレーム分析で薄くなった保護対象の破裂音を音源から補う。
func mixProtectedStopBursts(input manifest, prepared []preparedWorldUnit, wave []float64, sampleRate int) {
	if sampleRate <= 0 || len(wave) == 0 {
		return
	}
	sources := make(map[string]protectedStopSource)
	for index, item := range input.Units {
		if item.Speech == nil || !item.Speech.ProtectStop || index >= len(prepared) {
			continue
		}
		source, found := sources[item.Source]
		if !found {
			actualRate, samples, err := readPCM16(item.Source)
			if err != nil || actualRate != sampleRate || len(samples) == 0 {
				continue
			}
			source = protectedStopSource{sampleRate: actualRate, samples: samples,
				transient: highPass(samples, actualRate, 280)}
			sources[item.Source] = source
		}
		if item.Speech.CodaRelease {
			// 語末unitは短い終端範囲の外にpreutteranceを持つことがある。
			mixProtectedStopBurst(wave, source, worldSourceFrameBaseMS(item.OffsetMS)+prepared[index].cached.duration-22,
				item.PositionMS+item.LengthMS-22, 22, 4, item.Volume)
			continue
		}
		anchors, ok := worldSpeechAnchors(item, prepared[index].cached.duration)
		if !ok {
			continue
		}
		if anchors.coda {
			mixProtectedStopBurst(wave, source, worldSourceFrameBaseMS(item.OffsetMS)+anchors.sourceEnd-22,
				item.PositionMS+anchors.targetEnd-item.SkipMS-22, 22, 4, item.Volume)
			continue
		}
		mixProtectedStopBurst(wave, source, worldSourceFrameBaseMS(item.OffsetMS)+anchors.sourceOnset-8,
			item.PositionMS+anchors.targetOnset-item.SkipMS-8, 8, 32, item.Volume)
	}
}

func worldSourceFrameBaseMS(offsetMS float64) float64 {
	// WORLDの解析はoto.offsetより前のフレームから始まるため余りを二重加算しない。
	return math.Floor(math.Max(0, offsetMS)/worldFramePeriodMS) * worldFramePeriodMS
}

func highPass(samples []float64, sampleRate int, cutoffHz float64) []float64 {
	result := make([]float64, len(samples))
	if sampleRate <= 0 || cutoffHz <= 0 {
		copy(result, samples)
		return result
	}
	decay := math.Exp(-2 * math.Pi * cutoffHz / float64(sampleRate))
	low := 0.0
	for index, sample := range samples {
		low = decay*low + (1-decay)*sample
		result[index] = sample - low
	}
	return result
}

func mixProtectedStopBurst(wave []float64, source protectedStopSource, sourceStartMS, targetStartMS, preMS, postMS, volume float64) {
	if source.sampleRate <= 0 || len(source.samples) == 0 || len(source.transient) != len(source.samples) || preMS < 0 || postMS <= 0 {
		return
	}
	if volume <= 0 {
		volume = 100
	}
	volumeGain := math.Min(1.25, math.Max(0.25, volume/100))
	durationMS := preMS + postMS
	sourceDurationMS := float64(len(source.samples)) * 1000 / float64(source.sampleRate)
	if sourceStartMS < 0 {
		delta := -sourceStartMS
		sourceStartMS = 0
		targetStartMS += delta
		durationMS -= delta
	}
	if sourceStartMS >= sourceDurationMS || durationMS <= 0 {
		return
	}
	durationMS = math.Min(durationMS, sourceDurationMS-sourceStartMS)
	attackMS := math.Min(2, durationMS*.2)
	releaseMS := math.Min(5, durationMS*.3)
	for offsetMS := 0.0; offsetMS < durationMS; offsetMS += 1000 / float64(source.sampleRate) {
		sourceMS := sourceStartMS + offsetMS
		targetMS := targetStartMS + offsetMS
		sourceIndex := int(math.Round(sourceMS * float64(source.sampleRate) / 1000))
		targetIndex := int(math.Round(targetMS * float64(source.sampleRate) / 1000))
		if sourceIndex < 0 || sourceIndex >= len(source.samples) || targetIndex < 0 || targetIndex >= len(wave) {
			continue
		}
		weight := 1.0
		if attackMS > 0 && offsetMS < attackMS {
			weight = offsetMS / attackMS
		}
		remainingMS := durationMS - offsetMS
		if releaseMS > 0 && remainingMS < releaseMS {
			weight = math.Min(weight, remainingMS/releaseMS)
		}
		// 少量の原波形で気音を残し高域成分で破裂音の立ち上がりを補う。
		burst := .82*source.transient[sourceIndex] + .18*source.samples[sourceIndex]
		wave[targetIndex] += burst * weight * .8 * volumeGain
	}
}
