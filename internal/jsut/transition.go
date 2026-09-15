package jsut

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"

	"utautts/internal/acoustic"
	"utautts/internal/audio"
)

const TransitionExampleSchemaVersion = 1

const TransitionPriorVersion = 1

// TransitionExampleは自然な音素境界と擬似単独音境界の差分を表す。
type TransitionExample struct {
	SchemaVersion int               `json:"schema_version"`
	Kind          string            `json:"kind"`
	UtteranceID   string            `json:"utterance_id"`
	BoundaryIndex int               `json:"boundary_index"`
	LeftPhone     string            `json:"left_phone"`
	RightPhone    string            `json:"right_phone"`
	BoundaryMS    float64           `json:"boundary_ms"`
	FrameMS       float64           `json:"frame_ms"`
	StepMS        float64           `json:"step_ms"`
	LeftAnchor    Frame             `json:"left_anchor"`
	RightAnchor   Frame             `json:"right_anchor"`
	Frames        []TransitionFrame `json:"frames"`
}

// TransitionFrameは端点の線形補間から自然発話までの差分を保持する。
type TransitionFrame struct {
	OffsetMS           float64   `json:"offset_ms"`
	Natural            Frame     `json:"natural"`
	Baseline           Frame     `json:"baseline"`
	RMSResidualDB      float64   `json:"rms_residual_db"`
	F0ResidualCents    float64   `json:"f0_residual_cents,omitempty"`
	F0ResidualValid    bool      `json:"f0_residual_valid,omitempty"`
	SpectrumResidualDB []float64 `json:"spectrum_residual_db"`
}

type TransitionPrior struct {
	Version       int                                `json:"version"`
	Kind          string                             `json:"kind"`
	ID            string                             `json:"id"`
	PositionBins  int                                `json:"position_bins"`
	SpectrumBands int                                `json:"spectrum_bands"`
	ExampleCount  int                                `json:"example_count"`
	Global        TransitionPriorSequence            `json:"global"`
	Pairs         map[string]TransitionPriorSequence `json:"pairs"`
	Provenance    string                             `json:"provenance"`
	DataNotice    string                             `json:"data_notice"`
}

type TransitionPriorSequence struct {
	Count  int                    `json:"count"`
	Frames []TransitionPriorFrame `json:"frames"`
}

type TransitionPriorFrame struct {
	RMSResidualDB      float64   `json:"rms_residual_db"`
	F0ResidualCents    float64   `json:"f0_residual_cents,omitempty"`
	F0Count            int       `json:"f0_count,omitempty"`
	SpectrumResidualDB []float64 `json:"spectrum_residual_db"`
}

type TransitionMetrics struct {
	Frames              int     `json:"frames"`
	BaselineSpectrumMAE float64 `json:"baseline_spectrum_mae"`
	PriorSpectrumMAE    float64 `json:"prior_spectrum_mae"`
	BaselineRMSMAE      float64 `json:"baseline_rms_mae"`
	PriorRMSMAE         float64 `json:"prior_rms_mae"`
}

func BuildTransitionExamples(alignment Alignment, pcm *audio.PCM, windowMS, stepMS float64) ([]TransitionExample, error) {
	if err := alignment.Validate(); err != nil {
		return nil, err
	}
	if pcm == nil || pcm.SampleRate <= 0 || pcm.Channels <= 0 {
		return nil, fmt.Errorf("invalid pcm")
	}
	if windowMS < 20 || windowMS > 100 || stepMS < 2 || stepMS > 20 || stepMS >= windowMS {
		return nil, fmt.Errorf("invalid transition window or step")
	}
	wave := acoustic.Mono(pcm)
	result := make([]TransitionExample, 0, len(alignment.Boundaries))
	for index, boundary := range alignment.Boundaries {
		if !boundary.Trainable || !isJapaneseVowel(boundary.LeftPhone) || boundary.RightPhone == "sil" || boundary.RightPhone == "pau" {
			continue
		}
		leftPhone, rightPhone := alignment.Phones[index], alignment.Phones[index+1]
		half := math.Min(windowMS, math.Min(leftPhone.DurationMS*.45, rightPhone.DurationMS*.45))
		if half < 20 {
			continue
		}
		left, leftOK := frameAt(wave, pcm.SampleRate, boundary.BoundaryTimeMS-half, stepMS*2)
		right, rightOK := frameAt(wave, pcm.SampleRate, boundary.BoundaryTimeMS+half, stepMS*2)
		if !leftOK || !rightOK || len(left.SpectrumDB) != len(right.SpectrumDB) {
			continue
		}
		example := TransitionExample{
			SchemaVersion: TransitionExampleSchemaVersion, Kind: "jsut_cv_transition_residual",
			UtteranceID: alignment.ID, BoundaryIndex: index, LeftPhone: boundary.LeftPhone,
			RightPhone: boundary.RightPhone, BoundaryMS: boundary.BoundaryTimeMS,
			FrameMS: stepMS * 2, StepMS: stepMS, LeftAnchor: frameToJSON(left), RightAnchor: frameToJSON(right),
		}
		for offset := -half + stepMS; offset < half; offset += stepMS {
			natural, ok := frameAt(wave, pcm.SampleRate, boundary.BoundaryTimeMS+offset, stepMS*2)
			if !ok || len(natural.SpectrumDB) != len(left.SpectrumDB) {
				example.Frames = nil
				break
			}
			ratio := (offset + half) / (2 * half)
			baseline := interpolateFrame(left, right, ratio)
			item := TransitionFrame{OffsetMS: offset, Natural: frameToJSON(natural), Baseline: frameToJSON(baseline), RMSResidualDB: natural.RMSDB - baseline.RMSDB}
			item.SpectrumResidualDB = make([]float64, len(natural.SpectrumDB))
			for band := range item.SpectrumResidualDB {
				item.SpectrumResidualDB[band] = natural.SpectrumDB[band] - baseline.SpectrumDB[band]
			}
			if natural.F0Hz > 0 && baseline.F0Hz > 0 {
				item.F0ResidualValid = true
				item.F0ResidualCents = 1200 * math.Log2(natural.F0Hz/baseline.F0Hz)
			}
			example.Frames = append(example.Frames, item)
		}
		if len(example.Frames) >= 3 {
			result = append(result, example)
		}
	}
	return result, nil
}

func ReadTransitionExamples(path string) ([]TransitionExample, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := json.NewDecoder(bufio.NewReader(file))
	var result []TransitionExample
	for {
		var example TransitionExample
		if err := decoder.Decode(&example); err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode transition examples: %w", err)
		}
		if example.SchemaVersion != TransitionExampleSchemaVersion || example.Kind != "jsut_cv_transition_residual" || len(example.Frames) < 3 {
			return nil, fmt.Errorf("invalid transition example %s:%d", example.UtteranceID, example.BoundaryIndex)
		}
		result = append(result, example)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("transition dataset is empty")
	}
	return result, nil
}

func BuildTransitionPrior(examples []TransitionExample, bins int, id string) (*TransitionPrior, error) {
	if len(examples) == 0 || bins < 3 || bins > 64 {
		return nil, fmt.Errorf("invalid transition examples or bin count")
	}
	bands := len(examples[0].Frames[0].SpectrumResidualDB)
	if bands < 2 {
		return nil, fmt.Errorf("transition examples have no spectrum")
	}
	global := newTransitionAccumulator(bins, bands)
	pairs := make(map[string]*transitionAccumulator)
	for _, example := range examples {
		key := example.LeftPhone + "|" + example.RightPhone
		acc := pairs[key]
		if acc == nil {
			acc = newTransitionAccumulator(bins, bands)
			pairs[key] = acc
		}
		for index, frame := range example.Frames {
			if len(frame.SpectrumResidualDB) != bands {
				return nil, fmt.Errorf("inconsistent spectrum bands in %s", example.UtteranceID)
			}
			bin := positionBin(index, len(example.Frames), bins)
			global.add(bin, frame)
			acc.add(bin, frame)
		}
		global.count++
		acc.count++
	}
	if id == "" {
		id = "jsut-cv-transition-prior-v1"
	}
	prior := &TransitionPrior{Version: TransitionPriorVersion, Kind: "jsut_cv_transition_prior", ID: id,
		PositionBins: bins, SpectrumBands: bands, ExampleCount: len(examples), Global: global.result(),
		Pairs:      make(map[string]TransitionPriorSequence),
		Provenance: "Generated from JSUT BASIC5000 audio and jsut-label HTS labels; timing is Julius-estimated",
		DataNotice: "licenses/JSUT-DATA-AND-LABELS.txt"}
	for key, acc := range pairs {
		prior.Pairs[key] = acc.result()
	}
	return prior, nil
}

func EvaluateTransitionPrior(prior *TransitionPrior, examples []TransitionExample) TransitionMetrics {
	var result TransitionMetrics
	if prior == nil || prior.PositionBins < 3 {
		return result
	}
	for _, example := range examples {
		sequence, found := prior.Pairs[example.LeftPhone+"|"+example.RightPhone]
		if !found || sequence.Count < 8 {
			sequence = prior.Global
		}
		for index, frame := range example.Frames {
			bin := positionBin(index, len(example.Frames), prior.PositionBins)
			if bin >= len(sequence.Frames) {
				continue
			}
			prediction := sequence.Frames[bin]
			weight := math.Min(1, float64(sequence.Count)/32)
			for band, target := range frame.SpectrumResidualDB {
				if band >= len(prediction.SpectrumResidualDB) {
					break
				}
				result.BaselineSpectrumMAE += math.Abs(target)
				result.PriorSpectrumMAE += math.Abs(target - weight*prediction.SpectrumResidualDB[band])
			}
			result.BaselineRMSMAE += math.Abs(frame.RMSResidualDB)
			result.PriorRMSMAE += math.Abs(frame.RMSResidualDB - weight*prediction.RMSResidualDB)
			result.Frames++
		}
	}
	if result.Frames > 0 {
		bands := float64(prior.SpectrumBands * result.Frames)
		result.BaselineSpectrumMAE /= bands
		result.PriorSpectrumMAE /= bands
		result.BaselineRMSMAE /= float64(result.Frames)
		result.PriorRMSMAE /= float64(result.Frames)
	}
	return result
}

type transitionAccumulator struct {
	count  int
	frames []transitionFrameAccumulator
}

type transitionFrameAccumulator struct {
	rms      runningStat
	f0       runningStat
	spectrum []runningStat
}

func newTransitionAccumulator(bins, bands int) *transitionAccumulator {
	result := &transitionAccumulator{frames: make([]transitionFrameAccumulator, bins)}
	for index := range result.frames {
		result.frames[index].spectrum = make([]runningStat, bands)
	}
	return result
}

func (acc *transitionAccumulator) add(bin int, frame TransitionFrame) {
	target := &acc.frames[bin]
	target.rms.add(frame.RMSResidualDB)
	if frame.F0ResidualValid {
		target.f0.add(frame.F0ResidualCents)
	}
	for band, value := range frame.SpectrumResidualDB {
		target.spectrum[band].add(value)
	}
}

func (acc *transitionAccumulator) result() TransitionPriorSequence {
	result := TransitionPriorSequence{Count: acc.count, Frames: make([]TransitionPriorFrame, len(acc.frames))}
	for index, frame := range acc.frames {
		item := TransitionPriorFrame{RMSResidualDB: frame.rms.mean, F0ResidualCents: frame.f0.mean, F0Count: frame.f0.count, SpectrumResidualDB: make([]float64, len(frame.spectrum))}
		for band := range item.SpectrumResidualDB {
			item.SpectrumResidualDB[band] = frame.spectrum[band].mean
		}
		result.Frames[index] = item
	}
	return result
}

func positionBin(index, length, bins int) int {
	if length <= 1 {
		return bins / 2
	}
	return minInt(bins-1, int(math.Round(float64(index)*float64(bins-1)/float64(length-1))))
}

func interpolateFrame(left, right acoustic.Frame, ratio float64) acoustic.Frame {
	ratio = math.Max(0, math.Min(1, ratio))
	result := acoustic.Frame{Valid: left.Valid && right.Valid, RMSDB: left.RMSDB + ratio*(right.RMSDB-left.RMSDB)}
	if left.F0Hz > 0 && right.F0Hz > 0 {
		result.F0Hz = math.Exp(math.Log(left.F0Hz) + ratio*(math.Log(right.F0Hz)-math.Log(left.F0Hz)))
	}
	result.SpectrumDB = make([]float64, minInt(len(left.SpectrumDB), len(right.SpectrumDB)))
	for index := range result.SpectrumDB {
		result.SpectrumDB[index] = left.SpectrumDB[index] + ratio*(right.SpectrumDB[index]-left.SpectrumDB[index])
	}
	return result
}

func isJapaneseVowel(phone string) bool {
	switch phone {
	case "a", "i", "u", "e", "o":
		return true
	default:
		return false
	}
}
