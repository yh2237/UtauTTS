package voicebank

import (
	"math"
	"os"
	"sort"
	"utautts/internal/acoustic"
	"utautts/internal/audio"
	"utautts/internal/oto"
	"utautts/internal/pitch"
)

// SpeechProfileはoto.ini付近の音響特徴を保持する。
type SpeechProfile struct {
	Version                    int     `json:"version"`
	SourceSize                 int64   `json:"source_size"`
	SourceModTime              int64   `json:"source_mod_time"`
	TrimmedLengthMS            float64 `json:"trimmed_length_ms"`
	VowelTailMS                float64 `json:"vowel_tail_ms"`
	OriginalFixedMS            float64 `json:"original_fixed_ms"`
	SuggestedFixedMS           float64 `json:"suggested_fixed_ms"`
	StableStartMS              float64 `json:"stable_start_ms"`
	StableEndMS                float64 `json:"stable_end_ms"`
	TransientMS                float64 `json:"transient_ms,omitempty"`
	TransientConfidence        float64 `json:"transient_confidence,omitempty"`
	TransientDurationMS        float64 `json:"transient_duration_ms,omitempty"`
	ReleaseTransientMS         float64 `json:"release_transient_ms,omitempty"`
	ReleaseTransientConfidence float64 `json:"release_transient_confidence,omitempty"`
	ReleaseTransientDurationMS float64 `json:"release_transient_duration_ms,omitempty"`
	F0Hz                       float64 `json:"f0_hz"`
	RMSDB                      float64 `json:"rms_db"`
	VoicedRatio                float64 `json:"voiced_ratio"`
	Confidence                 float64 `json:"confidence"`
	Applied                    bool    `json:"applied"`
	Reason                     string  `json:"reason"`
}

// ClearSpeechProfilesは解析キャッシュを消去する。
func (b *Bank) ClearSpeechProfiles() {
	b.validationMu.Lock()
	defer b.validationMu.Unlock()
	b.speechProfiles = nil
}

func (b *Bank) CalibrateSpeech(entry oto.Entry) SpeechProfile {
	result := SpeechProfile{Version: 2, OriginalFixedMS: entry.Fixed, SuggestedFixedMS: entry.Fixed, Reason: "source-unavailable"}
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
	result.TrimmedLengthMS = float64(len(wave)) * 1000 / float64(rate)
	result.VowelTailMS = math.Max(0, result.TrimmedLengthMS-math.Max(0, entry.Fixed))
	// 解析時だけ8kHz付近まで間引く。
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
	result.TransientMS, result.TransientDurationMS, result.TransientConfidence = measureSpeechTransient(samples, sampleRate, entry.Preutterance)
	if entry.Fixed > entry.Preutterance+20 {
		result.ReleaseTransientMS, result.ReleaseTransientDurationMS, result.ReleaseTransientConfidence = measureSpeechTransient(samples, sampleRate, entry.Fixed)
	}
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

func measureSpeechTransient(samples []float64, sampleRate int, anchorMS float64) (float64, float64, float64) {
	if sampleRate <= 0 || len(samples) < 32 || anchorMS <= 0 {
		return 0, 0, 0
	}
	window := max(4, sampleRate*3/1000)
	hop := max(1, sampleRate/1000)
	center := int(anchorMS * float64(sampleRate) / 1000)
	left := max(1, center-sampleRate*45/1000)
	right := min(len(samples)-window, center+sampleRate*3/1000)
	if right <= left {
		return 0, 0, 0
	}
	values := make([]float64, 0, (right-left)/hop+1)
	bestStart, best := left, 0.0
	for start := left; start <= right; start += hop {
		sum := 0.0
		for index := start; index < start+window; index++ {
			delta := samples[index] - samples[index-1]
			sum += delta * delta
		}
		value := math.Sqrt(sum / float64(window))
		values = append(values, value)
		if value > best {
			best, bestStart = value, start
		}
	}
	if best <= 1e-8 || len(values) < 3 {
		return 0, 0, 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	baseline := sorted[len(sorted)/2]
	ratio := best / math.Max(1e-8, baseline)
	confidence := math.Max(0, math.Min(1, (ratio-1)/4))
	if confidence < .25 {
		return 0, 0, confidence
	}
	threshold := baseline + (best-baseline)*.35
	bestIndex := min(len(values)-1, max(0, (bestStart-left)/hop))
	first, last := bestIndex, bestIndex
	for first > 0 && values[first-1] >= threshold {
		first--
	}
	for last+1 < len(values) && values[last+1] >= threshold {
		last++
	}
	durationMS := float64((last-first)*hop+window) * 1000 / float64(sampleRate)
	return float64(bestStart+window/2) * 1000 / float64(sampleRate), durationMS, confidence
}
