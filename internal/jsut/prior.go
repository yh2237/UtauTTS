package jsut

import (
	"fmt"
	"math"
)

// PriorSchemaVersionはJSUT由来事前分布のスキーマ番号
const PriorSchemaVersion = 1

// ScalarStatsは実行時から独立した統計の要約
type ScalarStats struct {
	Count  int     `json:"count"`
	Mean   float64 `json:"mean"`
	Stddev float64 `json:"stddev"`
}

// FrameStatsは有効な音素中央フレームの要約
// F0は有声音だけを集計しVoicedRatioは全有効フレームで計算する
type FrameStats struct {
	Count       int           `json:"count"`
	RMSDB       ScalarStats   `json:"rms_db"`
	F0Hz        ScalarStats   `json:"f0_hz"`
	SpectrumDB  []ScalarStats `json:"spectrum_db,omitempty"`
	VoicedCount int           `json:"voiced_count"`
	VoicedRatio float64       `json:"voiced_ratio"`
}

// PhonePriorは1音素の長さと音響目標の分布
type PhonePrior struct {
	Count      int         `json:"count"`
	DurationMS ScalarStats `json:"duration_ms"`
	Frame      FrameStats  `json:"frame"`
}

// BoundaryPriorは1音素対の自然な隣接境界の要約
// 初期値や診断用でありUTAUの聴取ラベルの代わりにはしない
type BoundaryPrior struct {
	Count           int         `json:"count"`
	SpectrumDeltaDB ScalarStats `json:"spectrum_delta_db"`
	RMSDeltaDB      ScalarStats `json:"rms_delta_db"`
	F0DeltaCents    ScalarStats `json:"f0_delta_cents"`
	VoicingMismatch ScalarStats `json:"voicing_mismatch"`
}

// PriorはJSUT目標値の事前分布
// UTAU固有データへ適応するまで通常のRendererからは使わない
type Prior struct {
	Version         int                      `json:"version"`
	Kind            string                   `json:"kind"`
	ID              string                   `json:"id"`
	Description     string                   `json:"description,omitempty"`
	FeatureNames    []string                 `json:"feature_names"`
	Phones          map[string]PhonePrior    `json:"phones"`
	Contexts        map[string]PhonePrior    `json:"contexts,omitempty"`
	Boundaries      map[string]BoundaryPrior `json:"boundaries,omitempty"`
	Utterances      int                      `json:"utterances"`
	PhoneCount      int                      `json:"phone_count"`
	BoundaryCount   int                      `json:"boundary_count"`
	AnalysisFrameMS float64                  `json:"analysis_frame_ms,omitempty"`
	SpectrumBands   int                      `json:"spectrum_bands,omitempty"`
	Provenance      string                   `json:"provenance"`
	License         string                   `json:"license"`
	LicenseNotice   string                   `json:"license_notice"`
	DataNotice      string                   `json:"data_notice"`
}

// BuildPriorは発話記録の音素と自然境界の観測値を集計する
// 休止と無音は指定時だけ音素統計へ含める
func BuildPrior(records []Alignment, includePauses bool, id, description string) (*Prior, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("no JSUT alignment records")
	}
	if id == "" {
		id = "jsut-target-prior-v1"
	}
	if description == "" {
		description = "JSUT phone duration and acoustic target prior"
	}
	prior := &Prior{
		Version: PriorSchemaVersion, Kind: "jsut_target_prior", ID: id,
		Description: description,
		FeatureNames: []string{
			"duration_ms", "rms_db", "f0_hz", "spectrum_db", "voiced_ratio",
			"boundary_spectrum_delta_db", "boundary_rms_delta_db",
			"boundary_f0_delta_cents", "boundary_voicing_mismatch",
		},
		Phones: make(map[string]PhonePrior), Contexts: make(map[string]PhonePrior),
		Boundaries: make(map[string]BoundaryPrior), Utterances: len(records),
		Provenance:    "Generated from JSUT BASIC5000 audio and jsut-label HTS labels; timing is Julius-estimated",
		License:       "UtauTTS project policy: academic research, non-commercial research, and personal use only; commercial use requires prior permission from the JSUT rights holders",
		LicenseNotice: "licenses/PROSODY-MODELS.txt",
		DataNotice:    "licenses/JSUT-DATA-AND-LABELS.txt",
	}
	phoneAccumulators := make(map[string]*phoneAccumulator)
	contextAccumulators := make(map[string]*phoneAccumulator)
	boundaryAccumulators := make(map[string]*boundaryAccumulator)
	for recordIndex, record := range records {
		if err := record.Validate(); err != nil {
			return nil, fmt.Errorf("record %d (%s): %w", recordIndex+1, record.ID, err)
		}
		if record.AnalysisFrameMS > 0 {
			if prior.AnalysisFrameMS == 0 {
				prior.AnalysisFrameMS = record.AnalysisFrameMS
				prior.SpectrumBands = record.SpectrumBands
			} else if math.Abs(prior.AnalysisFrameMS-record.AnalysisFrameMS) > 1e-9 || prior.SpectrumBands != record.SpectrumBands {
				return nil, fmt.Errorf("record %d (%s) uses a different acoustic analysis window", recordIndex+1, record.ID)
			}
		}
		for _, phone := range record.Phones {
			if !includePauses && (phone.Silence || phone.Pause) {
				continue
			}
			acc := phoneAccumulators[phone.Symbol]
			if acc == nil {
				acc = &phoneAccumulator{}
				phoneAccumulators[phone.Symbol] = acc
			}
			acc.add(phone)
			contextKey := phone.Previous + "|" + phone.Symbol + "|" + phone.Next
			contextAcc := contextAccumulators[contextKey]
			if contextAcc == nil {
				contextAcc = &phoneAccumulator{}
				contextAccumulators[contextKey] = contextAcc
			}
			contextAcc.add(phone)
			prior.PhoneCount++
		}
		for _, boundary := range record.Boundaries {
			if !boundary.Trainable || boundary.Features == nil {
				continue
			}
			key := boundary.LeftPhone + "|" + boundary.RightPhone
			acc := boundaryAccumulators[key]
			if acc == nil {
				acc = &boundaryAccumulator{}
				boundaryAccumulators[key] = acc
			}
			acc.add(*boundary.Features)
			prior.BoundaryCount++
		}
	}
	for key, acc := range phoneAccumulators {
		prior.Phones[key] = acc.result()
	}
	for key, acc := range contextAccumulators {
		prior.Contexts[key] = acc.result()
	}
	for key, acc := range boundaryAccumulators {
		prior.Boundaries[key] = acc.result()
	}
	if len(prior.Phones) == 0 {
		return nil, fmt.Errorf("no usable phones in JSUT alignment records")
	}
	return prior, nil
}

type runningStat struct {
	count int
	mean  float64
	m2    float64
}

func (stat *runningStat) add(value float64) {
	if !finite(value) {
		return
	}
	stat.count++
	delta := value - stat.mean
	stat.mean += delta / float64(stat.count)
	stat.m2 += delta * (value - stat.mean)
}

func (stat runningStat) result() ScalarStats {
	stddev := 0.0
	if stat.count > 1 {
		stddev = math.Sqrt(math.Max(0, stat.m2/float64(stat.count-1)))
	}
	return ScalarStats{Count: stat.count, Mean: stat.mean, Stddev: stddev}
}

type frameAccumulator struct {
	count       int
	rms         runningStat
	f0          runningStat
	spectrum    []runningStat
	voicedCount int
}

func (acc *frameAccumulator) add(frame *Frame) {
	if frame == nil || !frame.Valid {
		return
	}
	acc.count++
	acc.rms.add(frame.RMSDB)
	if frame.F0Hz > 0 && finite(frame.F0Hz) {
		acc.f0.add(frame.F0Hz)
		acc.voicedCount++
	}
	if len(acc.spectrum) < len(frame.SpectrumDB) {
		acc.spectrum = append(acc.spectrum, make([]runningStat, len(frame.SpectrumDB)-len(acc.spectrum))...)
	}
	for index, value := range frame.SpectrumDB {
		acc.spectrum[index].add(value)
	}
}

func (acc frameAccumulator) result() FrameStats {
	spectrum := make([]ScalarStats, len(acc.spectrum))
	for index, stat := range acc.spectrum {
		spectrum[index] = stat.result()
	}
	ratio := 0.0
	if acc.count > 0 {
		ratio = float64(acc.voicedCount) / float64(acc.count)
	}
	return FrameStats{Count: acc.count, RMSDB: acc.rms.result(), F0Hz: acc.f0.result(), SpectrumDB: spectrum, VoicedCount: acc.voicedCount, VoicedRatio: ratio}
}

type phoneAccumulator struct {
	count    int
	duration runningStat
	frame    frameAccumulator
}

func (acc *phoneAccumulator) add(phone Phone) {
	acc.count++
	acc.duration.add(phone.DurationMS)
	acc.frame.add(phone.Frame)
}

func (acc phoneAccumulator) result() PhonePrior {
	return PhonePrior{Count: acc.count, DurationMS: acc.duration.result(), Frame: acc.frame.result()}
}

type boundaryAccumulator struct {
	count           int
	spectrumDelta   runningStat
	rmsDelta        runningStat
	f0Delta         runningStat
	voicingMismatch runningStat
}

func (acc *boundaryAccumulator) add(features BoundaryFeatures) {
	acc.count++
	acc.spectrumDelta.add(features.SpectrumDeltaDB)
	acc.rmsDelta.add(features.RMSDeltaDB)
	if features.F0Comparable {
		acc.f0Delta.add(features.F0DeltaCents)
	}
	if features.VoicingMismatch {
		acc.voicingMismatch.add(1)
	} else {
		acc.voicingMismatch.add(0)
	}
}

func (acc boundaryAccumulator) result() BoundaryPrior {
	return BoundaryPrior{Count: acc.count, SpectrumDeltaDB: acc.spectrumDelta.result(), RMSDeltaDB: acc.rmsDelta.result(), F0DeltaCents: acc.f0Delta.result(), VoicingMismatch: acc.voicingMismatch.result()}
}
