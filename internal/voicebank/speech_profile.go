package voicebank

import (
	"math"
	"os"
	"utautts/internal/acoustic"
	"utautts/internal/audio"
	"utautts/internal/oto"
	"utautts/internal/pitch"
)

// SpeechProfile is a bounded acoustic search around oto, not forced alignment.
type SpeechProfile struct {
	Version          int     `json:"version"`
	SourceSize       int64   `json:"source_size"`
	SourceModTime    int64   `json:"source_mod_time"`
	OriginalFixedMS  float64 `json:"original_fixed_ms"`
	SuggestedFixedMS float64 `json:"suggested_fixed_ms"`
	StableStartMS    float64 `json:"stable_start_ms"`
	StableEndMS      float64 `json:"stable_end_ms"`
	F0Hz             float64 `json:"f0_hz"`
	RMSDB            float64 `json:"rms_db"`
	VoicedRatio      float64 `json:"voiced_ratio"`
	Confidence       float64 `json:"confidence"`
	Applied          bool    `json:"applied"`
	Reason           string  `json:"reason"`
}

// Reloading a bank also clears the cache. It holds at most 4096 scalar profiles.
func (b *Bank) ClearSpeechProfiles() {
	b.validationMu.Lock()
	defer b.validationMu.Unlock()
	b.speechProfiles = nil
}

func (b *Bank) CalibrateSpeech(entry oto.Entry) SpeechProfile {
	result := SpeechProfile{Version: 1, OriginalFixedMS: entry.Fixed, SuggestedFixedMS: entry.Fixed, Reason: "source-unavailable"}
	info, err := os.Stat(entry.Filename)
	if err != nil {
		return result
	}
	result.SourceSize = info.Size()
	result.SourceModTime = info.ModTime().UnixNano()
	b.validationMu.Lock()
	cached, ok := b.speechProfiles[entry]
	b.validationMu.Unlock()
	if ok && cached.SourceSize == result.SourceSize && cached.SourceModTime == result.SourceModTime {
		return cached
	}
	pcm, err := audio.ReadWav(entry.Filename)
	if err == nil {
		pcm, err = audio.TrimPCM(pcm, entry.Offset, entry.Blank)
	}
	if err == nil {
		result = measureSpeechProfile(acoustic.Mono(pcm), pcm.SampleRate, entry, result)
	}
	b.validationMu.Lock()
	defer b.validationMu.Unlock()
	if b.speechProfiles == nil || len(b.speechProfiles) >= 4096 {
		b.speechProfiles = make(map[oto.Entry]SpeechProfile)
	}
	b.speechProfiles[entry] = result
	return result
}

func measureSpeechProfile(wave []float64, rate int, entry oto.Entry, result SpeechProfile) SpeechProfile {
	result.Reason = "no-stable-voicing"
	if rate <= 0 || len(wave) < 32 || math.IsNaN(entry.Fixed) || math.IsInf(entry.Fixed, 0) {
		return result
	}
	// Downsample for analysis only; the renderer retains the original samples.
	stride := max(1, rate/8000)
	samples := make([]float64, 0, len(wave)/stride)
	for i := 0; i+stride <= len(wave); i += stride {
		sum := 0.0
		for _, v := range wave[i : i+stride] {
			sum += v
		}
		samples = append(samples, sum/float64(stride))
	}
	sampleRate := rate / stride
	frame := max(16, sampleRate*40/1000)
	hop := max(1, sampleRate*5/1000)
	left := int(math.Max(entry.Preutterance, entry.Fixed-25) * float64(sampleRate) / 1000)
	right := min(len(samples)-frame, int((entry.Fixed+120)*float64(sampleRate)/1000))
	count, voiced, run := 0, 0, 0
	priorF0, priorDB := 0.0, 0.0
	bestStart, bestEnd := -1, -1
	sumF0, sumDB := 0.0, 0.0
	for start := max(0, left); start <= right; start += hop {
		segment := samples[start : start+frame]
		f0 := pitch.Estimate(segment, sampleRate)
		db := acoustic.DB(acoustic.RMS(segment))
		count++
		stable := f0 > 0 && db > -50
		if stable {
			voiced++
			sumF0 += f0
			sumDB += db
		}
		if stable && (run == 0 || (math.Abs(1200*math.Log2(f0/priorF0)) < 120 && math.Abs(db-priorDB) < 4)) {
			run++
		} else if stable {
			run = 1
		} else {
			run = 0
		}
		if run >= 3 && bestStart < 0 {
			bestStart = start - 2*hop
		}
		if bestStart >= 0 {
			if run < 3 {
				break
			}
			bestEnd = start + frame
		}
		priorF0, priorDB = f0, db
	}
	if count > 0 {
		result.VoicedRatio = float64(voiced) / float64(count)
	}
	if voiced > 0 {
		result.F0Hz = sumF0 / float64(voiced)
		result.RMSDB = sumDB / float64(voiced)
	}
	if bestStart < 0 {
		return result
	}
	result.StableStartMS = float64(bestStart) * 1000 / float64(sampleRate)
	result.StableEndMS = float64(bestEnd) * 1000 / float64(sampleRate)
	result.Confidence = result.VoicedRatio * math.Min(1, (result.StableEndMS-result.StableStartMS)/60)
	result.SuggestedFixedMS = math.Max(entry.Preutterance, math.Max(entry.Fixed-20, math.Min(entry.Fixed+20, result.StableStartMS+20)))
	result.Applied = result.Confidence >= 0.8 && entry.Fixed >= entry.Preutterance && result.SuggestedFixedMS <= float64(len(wave))*1000/float64(rate)-30
	if result.Applied {
		result.Reason = "stable-voicing-near-oto"
	} else {
		result.SuggestedFixedMS = entry.Fixed
		result.Reason = "low-confidence"
	}
	return result
}
