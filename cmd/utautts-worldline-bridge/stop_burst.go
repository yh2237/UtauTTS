package main

import "math"

// 音源波形と破裂音補強用の高域成分を保持する。
type protectedStopSource struct {
	sampleRate int
	samples    []float64
	transient  []float64
}

// WORLDのフレーム分析で薄くなった保護対象の破裂音を音源から補う。
func mixProtectedStopBursts(input manifest, prepared []preparedWorldUnit, wave []float64, sampleRate int) map[int]float64 {
	result := make(map[int]float64)
	if sampleRate <= 0 || len(wave) == 0 {
		return result
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
		if item.Speech.PreserveStopOnly {
			sourceOnset, targetOnset := stopBurstOnsets(item, worldSourceExactBaseMS(item.OffsetMS), item.Speech.SourceOnsetMS, item.Speech.TargetOnsetMS)
			postMS := stopTransientPostMS(item.Speech.SourceTransientDurationMS, 32)
			result[item.Speech.UnitIndex] = mixProtectedStopBurst(wave, source, sourceOnset-8,
				targetOnset-8, 8, postMS, item.Volume*worldUnitEnergy(item))
			continue
		}
		if item.Speech.CodaRelease {
			if item.Speech.SeparateRelease && item.Speech.ReleaseMS > 0 {
				// E2a: 解放は末尾の短い非伸縮区間に置き、原音の高域をそこへ加算する。
				releaseMS := item.Speech.ReleaseMS
				targetStart := item.PositionMS + item.LengthMS - releaseMS
				sourceStart := worldSourceExactBaseMS(item.OffsetMS) + math.Max(0, prepared[index].cached.duration-releaseMS-6)
				if item.Speech.SourceTransientMS > 0 {
					// 測定した解放過渡を優先し、末尾の減衰へずれないようにする。
					sourceStart = worldSourceExactBaseMS(item.OffsetMS) + item.Speech.SourceTransientMS - 8
				}
				preMS, postMS := releaseMS*0.25, releaseMS*0.75
				result[item.Speech.UnitIndex] = mixProtectedStopBurst(wave, source, sourceStart,
					targetStart, preMS, postMS, item.Volume*worldUnitEnergy(item))
				continue
			}
			sourceStart := worldSourceFrameBaseMS(item.OffsetMS) + prepared[index].cached.duration - 22
			targetEnd := item.PositionMS + item.LengthMS
			preMS, postMS := 22.0, 4.0
			if item.Speech.SourceTransientMS > 0 {
				sourceStart = worldSourceExactBaseMS(item.OffsetMS) + item.Speech.SourceTransientMS - 8
				preMS, postMS = 8, stopTransientPostMS(item.Speech.SourceTransientDurationMS, 14)
			}
			result[item.Speech.UnitIndex] = mixProtectedStopBurst(wave, source, sourceStart,
				targetEnd-preMS-4, preMS, postMS, item.Volume*worldUnitEnergy(item))
			continue
		}
		anchors, ok := worldSpeechAnchors(item, prepared[index].cached.duration)
		if !ok {
			continue
		}
		if anchors.coda {
			result[item.Speech.UnitIndex] = mixProtectedStopBurst(wave, source, worldSourceFrameBaseMS(item.OffsetMS)+anchors.sourceEnd-22,
				item.PositionMS+anchors.targetEnd-item.SkipMS-22, 22, 4, item.Volume*worldUnitEnergy(item))
			continue
		}
		sourceOnset, targetOnset := stopBurstOnsets(item, worldSourceFrameBaseMS(item.OffsetMS), anchors.sourceOnset, anchors.targetOnset)
		postMS := stopTransientPostMS(item.Speech.SourceTransientDurationMS, 32)
		result[item.Speech.UnitIndex] = mixProtectedStopBurst(wave, source, sourceOnset-8,
			targetOnset-8, 8, postMS, item.Volume*worldUnitEnergy(item))
	}
	return result
}

func stopTransientPostMS(durationMS, fallback float64) float64 {
	if durationMS <= 0 || math.IsNaN(durationMS) || math.IsInf(durationMS, 0) {
		return fallback
	}
	return math.Max(10, math.Min(24, durationMS+6))
}

func stopBurstOnsets(item unit, sourceBaseMS, sourceAnchorMS, targetAnchorMS float64) (float64, float64) {
	sourceOnset := sourceAnchorMS
	targetOnset := item.PositionMS + targetAnchorMS - item.SkipMS
	if item.Speech.SourceTransientMS > 0 {
		delta := item.Speech.SourceTransientMS - item.Speech.SourceOnsetMS
		sourceBaseMS = worldSourceExactBaseMS(item.OffsetMS)
		sourceOnset = item.Speech.SourceTransientMS
		targetOnset += delta
	}
	return sourceBaseMS + sourceOnset, targetOnset
}

func worldUnitEnergy(item unit) float64 {
	energy := item.EnergyFactor
	if energy <= 0 || math.IsNaN(energy) || math.IsInf(energy, 0) {
		return 1
	}
	return math.Min(1.5, energy)
}

func worldSourceFrameBaseMS(offsetMS float64) float64 {
	// WORLDの解析はoto.offsetより前のフレームから始まるため余りを二重加算しない。
	return math.Floor(math.Max(0, offsetMS)/worldFramePeriodMS) * worldFramePeriodMS
}

func worldSourceExactBaseMS(offsetMS float64) float64 {
	return math.Max(0, offsetMS)
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

func mixProtectedStopBurst(wave []float64, source protectedStopSource, sourceStartMS, targetStartMS, preMS, postMS, volume float64) float64 {
	if source.sampleRate <= 0 || len(source.samples) == 0 || len(source.transient) != len(source.samples) || preMS < 0 || postMS <= 0 {
		return 0
	}
	if volume < 0 {
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
		return 0
	}
	durationMS = math.Min(durationMS, sourceDurationMS-sourceStartMS)
	locality := stopBurstLocality(source.transient, source.sampleRate, sourceStartMS, durationMS)
	burstGain := .65 + .35*math.Max(0, math.Min(1, (locality-1)/2.5))
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
		burst := .92*source.transient[sourceIndex] + .08*source.samples[sourceIndex]
		wave[targetIndex] += burst * weight * .8 * volumeGain * burstGain
	}
	return .8 * volumeGain * burstGain
}

func stopBurstLocality(samples []float64, sampleRate int, startMS, durationMS float64) float64 {
	if sampleRate <= 0 || len(samples) == 0 || durationMS <= 0 {
		return 0
	}
	start := min(len(samples), max(0, int(math.Round(startMS*float64(sampleRate)/1000))))
	end := min(len(samples), max(start, int(math.Round((startMS+durationMS)*float64(sampleRate)/1000))))
	margin := max(1, sampleRate*50/1000)
	guard := max(1, sampleRate*5/1000)
	energy := func(left, right int) float64 {
		left, right = max(0, left), min(len(samples), right)
		if right <= left {
			return 0
		}
		sum := 0.0
		for _, sample := range samples[left:right] {
			sum += sample * sample
		}
		return math.Sqrt(sum / float64(right-left))
	}
	local := energy(start, end)
	before := energy(start-margin, start-guard)
	after := energy(end+guard, end+margin)
	background := math.Max(before, after)
	if local <= 1e-8 {
		return 0
	}
	return local / math.Max(1e-6, background)
}
