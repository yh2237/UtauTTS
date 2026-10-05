package voicebank

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"utautts/internal/acoustic"
	"utautts/internal/oto"
	"utautts/internal/sourceaudio"
)

// 音響観測だけを持ち、音素名は推定しない。
type SourceFrame struct {
	StartMS          float64 `json:"start_ms"`
	EndMS            float64 `json:"end_ms"`
	RMSDB            float64 `json:"rms_dbfs"`
	PeakDB           float64 `json:"peak_dbfs"`
	ZeroCrossingRate float64 `json:"zero_crossing_rate"`
	Periodicity      float64 `json:"periodicity"`
	Evidence         string  `json:"evidence"`
}

type SourceRegion struct {
	StartMS  float64 `json:"start_ms"`
	EndMS    float64 `json:"end_ms"`
	Evidence string  `json:"evidence"`
}

type SourceLandmark struct {
	Kind             string  `json:"kind"`
	SourceMS         float64 `json:"source_ms"`
	DurationMS       float64 `json:"duration_ms"`
	HeuristicScore   float64 `json:"heuristic_score"`
	LocalRMSDB       float64 `json:"local_rms_dbfs"`
	RelativeToPeakDB float64 `json:"relative_to_peak_db"`
}

type SourceAnalysis struct {
	Version              int              `json:"version"`
	Status               string           `json:"status"`
	TimeOrigin           string           `json:"time_origin"`
	SourceSHA256         string           `json:"source_sha256"`
	SampleRate           int              `json:"sample_rate"`
	DurationMS           float64          `json:"duration_ms"`
	FrameMS              float64          `json:"frame_ms"`
	WindowMS             float64          `json:"window_ms"`
	PeriodicityMethod    string           `json:"periodicity_method"`
	LowEnergyThresholdDB float64          `json:"low_energy_threshold_dbfs"`
	Frames               []SourceFrame    `json:"frames"`
	Regions              []SourceRegion   `json:"regions"`
	Landmarks            []SourceLandmark `json:"landmarks"`
	Warnings             []string         `json:"warnings"`
}

func (b *Bank) AnalyzeSpeechSource(entry oto.Entry) (SourceAnalysis, error) {
	pcm, x, err := sourceaudio.TrimmedMono(entry.Filename, entry.Offset, entry.Blank)
	if err != nil {
		return SourceAnalysis{}, err
	}
	if len(x) < 32 || pcm.SampleRate <= 0 {
		return SourceAnalysis{}, fmt.Errorf("source too short for acoustic analysis")
	}
	a := analyzeSourceFrames(x, pcm.SampleRate)
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "v1/%d/%d/", pcm.SampleRate, pcm.Channels)
	for _, sample := range pcm.Data {
		_, _ = h.Write([]byte{byte(sample), byte(uint16(sample) >> 8)})
	}
	a.SourceSHA256 = hex.EncodeToString(h.Sum(nil))
	p := b.CalibrateSpeech(entry)
	for _, candidate := range []SourceLandmark{
		{Kind: "preutterance-transient", SourceMS: p.TransientMS, DurationMS: p.TransientDurationMS, HeuristicScore: p.TransientConfidence},
		{Kind: "fixed-transient", SourceMS: p.ReleaseTransientMS, DurationMS: p.ReleaseTransientDurationMS, HeuristicScore: p.ReleaseTransientConfidence},
	} {
		if candidate.SourceMS <= 0 || candidate.SourceMS >= a.DurationMS || candidate.DurationMS <= 0 {
			continue
		}
		left := max(0, int((candidate.SourceMS-4)*float64(pcm.SampleRate)/1000))
		right := min(len(x), int((candidate.SourceMS+candidate.DurationMS+6)*float64(pcm.SampleRate)/1000))
		candidate.LocalRMSDB = acoustic.DB(acoustic.RMS(x[left:right]))
		peak := -140.0
		for _, frame := range a.Frames {
			peak = max(peak, frame.RMSDB)
		}
		candidate.RelativeToPeakDB = candidate.LocalRMSDB - peak
		a.Landmarks = append(a.Landmarks, candidate)
		if candidate.RelativeToPeakDB < -25 {
			a.Warnings = append(a.Warnings, candidate.Kind+":weak-relative-to-source")
		}
	}
	return a, nil
}

func analyzeSourceFrames(x []float64, rate int) SourceAnalysis {
	a := SourceAnalysis{Version: 2, Status: "unverified-acoustic-observation", TimeOrigin: "oto-offset",
		SampleRate: rate, DurationMS: float64(len(x)) * 1000 / float64(rate), FrameMS: 10, WindowMS: 20,
		PeriodicityMethod: "normalized-autocorrelation-local-peak-80-500hz-v2",
		Frames:            []SourceFrame{}, Regions: []SourceRegion{}, Landmarks: []SourceLandmark{}, Warnings: []string{}}
	hop, window := max(1, rate/100), max(1, rate/50)
	peak := -140.0
	for start := 0; start < len(x); start += hop {
		end := min(len(x), start+window)
		segment := x[start:end]
		f := SourceFrame{StartMS: float64(start) * 1000 / float64(rate), EndMS: float64(min(len(x), start+hop)) * 1000 / float64(rate), RMSDB: acoustic.DB(acoustic.RMS(segment))}
		maximum, crossings := 0.0, 0
		for i, v := range segment {
			maximum = max(maximum, math.Abs(v))
			if i > 0 && (v >= 0) != (segment[i-1] >= 0) {
				crossings++
			}
		}
		f.PeakDB = acoustic.DB(maximum)
		f.ZeroCrossingRate = float64(crossings) / float64(max(1, len(segment)-1))
		// 周期性には40msを使い、短いレベル窓と区別する。
		f.Periodicity = sourcePeriodicity(x[start:min(len(x), start+rate/25)], rate)
		a.Frames = append(a.Frames, f)
		peak = max(peak, f.RMSDB)
	}
	a.LowEnergyThresholdDB = max(-65, peak-35)
	for i := range a.Frames {
		f := &a.Frames[i]
		switch {
		case f.RMSDB < a.LowEnergyThresholdDB:
			f.Evidence = "low-energy"
		case f.Periodicity >= .65:
			f.Evidence = "periodic"
		case f.Periodicity < .4 && f.ZeroCrossingRate > .15:
			f.Evidence = "aperiodic-high-crossing"
		default:
			f.Evidence = "mixed-or-uncertain"
		}
		if len(a.Regions) > 0 && a.Regions[len(a.Regions)-1].Evidence == f.Evidence {
			a.Regions[len(a.Regions)-1].EndMS = f.EndMS
		} else {
			a.Regions = append(a.Regions, SourceRegion{StartMS: f.StartMS, EndMS: f.EndMS, Evidence: f.Evidence})
		}
	}
	if peak < -65 {
		a.Warnings = append(a.Warnings, "source-near-silence")
	}
	lastRise := -100.0
	for i := 3; i < len(a.Frames); i++ {
		f := a.Frames[i]
		previous := max(a.Frames[i-3].RMSDB, a.Frames[i-2].RMSDB)
		rise := f.RMSDB - previous
		// 音素名を付けず、otoに依存しない立ち上がり候補を残す。
		if rise >= 6 && f.RMSDB >= peak-35 && f.StartMS-lastRise >= 30 {
			a.Landmarks = append(a.Landmarks, SourceLandmark{Kind: "energy-rise", SourceMS: f.StartMS,
				DurationMS: a.WindowMS, HeuristicScore: min(1, rise/18), LocalRMSDB: f.RMSDB, RelativeToPeakDB: f.RMSDB - peak})
			lastRise = f.StartMS
		}
	}
	return a
}

func sourcePeriodicity(x []float64, rate int) float64 {
	// 間引きは周期性の測定だけに使う。
	stride := max(1, rate/8000)
	y := make([]float64, 0, len(x)/stride)
	for i := 0; i+stride <= len(x); i += stride {
		sum := 0.0
		for _, v := range x[i : i+stride] {
			sum += v
		}
		y = append(y, sum/float64(stride))
	}
	r := rate / stride
	if len(y) < r/50 {
		return 0
	}
	mean := 0.0
	for _, v := range y {
		mean += v
	}
	mean /= float64(len(y))
	for i := range y {
		y[i] -= mean
	}
	first, last := max(1, r/500), min(r/80, len(y)/2)
	correlations := make([]float64, last+2)
	correlations[0] = 1
	for lag := max(1, first-1); lag <= last+1; lag++ {
		cross, left, right := 0.0, 0.0, 0.0
		for i := lag; i < len(y); i++ {
			a, b := y[i], y[i-lag]
			cross += a * b
			left += a * a
			right += b * b
		}
		if left*right > 1e-16 {
			correlations[lag] = cross / math.Sqrt(left*right)
		}
	}
	// 低周波の揺れや傾きによる単調な相関を除く。
	best := 0.0
	for lag := first; lag <= last; lag++ {
		value := correlations[lag]
		if value > correlations[lag-1]+1e-6 && value > correlations[lag+1]+1e-6 {
			best = max(best, value)
		}
	}
	return min(1, best)
}
