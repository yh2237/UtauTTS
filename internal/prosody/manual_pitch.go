package prosody

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

const ManualPitchVersion = 1

// ManualPitchCentsLimitは、自動予測値に加える補正値の上限。
// GUIのピッチ欄は表示上の値を±800centに収めるが、補正値は自動予測値との差なので、
// 自動予測値が大きいと±800centを超える。
const ManualPitchCentsLimit = 4800

// ManualPitchFileはモデル輪郭に対する相対補正を保持する。
// framesモードではモーラ補正にフレーム補正を加える。FrameMSが0なら発話全体へ伸縮する。
type ManualPitchFile struct {
	Version int                `json:"version"`
	Reading string             `json:"reading,omitempty"`
	Mode    string             `json:"mode,omitempty"`
	Points  []ManualPitchPoint `json:"points,omitempty"`
	Frames  []float64          `json:"frames,omitempty"`
	FrameMS float64            `json:"frame_ms,omitempty"`
}

type ManualPitchPoint struct {
	Position int     `json:"position"`
	Mora     string  `json:"mora,omitempty"`
	Cents    float64 `json:"cents"`
}

func LoadManualPitch(path string) (*ManualPitchFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file ManualPitchFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("decode manual pitch file: %w", err)
	}
	if err := file.Validate(); err != nil {
		return nil, err
	}
	return &file, nil
}

func (file *ManualPitchFile) Validate() error {
	if file == nil {
		return nil
	}
	if file.Version == 0 {
		return fmt.Errorf("manual pitch version is required")
	}
	if file.Version != ManualPitchVersion {
		return fmt.Errorf("unsupported manual pitch version %d", file.Version)
	}
	if file.Mode == "" {
		file.Mode = "offset"
	}
	if file.Mode != "offset" && file.Mode != "replace" && file.Mode != "frames" {
		return fmt.Errorf("unsupported manual pitch mode %q", file.Mode)
	}
	for index, point := range file.Points {
		if point.Position < 0 {
			return fmt.Errorf("manual pitch point %d has negative position", index)
		}
		if math.IsNaN(point.Cents) || math.IsInf(point.Cents, 0) || math.Abs(point.Cents) > ManualPitchCentsLimit {
			return fmt.Errorf("manual pitch point %d is outside +/-%d cents", index, ManualPitchCentsLimit)
		}
	}
	if file.Mode == "frames" {
		if math.IsNaN(file.FrameMS) || math.IsInf(file.FrameMS, 0) || file.FrameMS < 0 || (file.FrameMS > 0 && file.FrameMS < 1) {
			return fmt.Errorf("manual pitch frame_ms must be 0 or at least 1 millisecond")
		}
		if len(file.Frames) > 20000 {
			return fmt.Errorf("manual pitch frames are limited to 20000 entries")
		}
		for index, cents := range file.Frames {
			if math.IsNaN(cents) || math.IsInf(cents, 0) || math.Abs(cents) > ManualPitchCentsLimit {
				return fmt.Errorf("manual pitch frame %d is outside +/-%d cents", index, ManualPitchCentsLimit)
			}
		}
	}
	return nil
}

// Curveはモーラ補正とフレーム補正を同じ時間軸へ補間して合算する。
func (file *ManualPitchFile) Curve(morae []linguistics.Mora, timings []MoraTiming, durationMS float64) (*PitchContour, error) {
	if file == nil {
		return nil, nil
	}
	frameMS := 10.0
	if file.Mode == "frames" && file.FrameMS > 0 {
		frameMS = min(frameMS, file.FrameMS)
	}
	curve, err := file.pointCurve(morae, timings, durationMS, frameMS)
	if err != nil || file.Mode != "frames" {
		return curve, err
	}
	frames, err := file.frameCurve(durationMS, frameMS)
	if err != nil || curve == nil {
		return frames, err
	}
	for index := range curve.Cents {
		curve.Cents[index] += frames.Cents[index]
	}
	return curve, nil
}

func (file *ManualPitchFile) pointCurve(morae []linguistics.Mora, timings []MoraTiming, durationMS, frameMS float64) (*PitchContour, error) {
	if len(file.Points) == 0 {
		return nil, nil
	}
	if len(timings) != len(morae) {
		return nil, fmt.Errorf("manual pitch timing count %d does not match mora count %d", len(timings), len(morae))
	}
	if durationMS <= 0 {
		return nil, fmt.Errorf("manual pitch duration must be positive")
	}
	values := make([]float64, len(morae))
	set := make([]bool, len(morae))
	for _, point := range file.Points {
		if point.Position < 0 || point.Position >= len(morae) {
			return nil, fmt.Errorf("manual pitch point position %d is outside mora count %d", point.Position, len(morae))
		}
		if morae[point.Position].Pause {
			return nil, fmt.Errorf("manual pitch point position %d refers to a pause", point.Position)
		}
		if set[point.Position] {
			return nil, fmt.Errorf("manual pitch point position %d is duplicated", point.Position)
		}
		if point.Mora != "" && point.Mora != morae[point.Position].Text {
			return nil, fmt.Errorf("manual pitch point %d mora %q does not match %q", point.Position, point.Mora, morae[point.Position].Text)
		}
		values[point.Position] = point.Cents
		set[point.Position] = true
	}
	centers := make([]float64, 0, len(morae))
	centerValues := make([]float64, 0, len(morae))
	for index, mora := range morae {
		if mora.Pause {
			continue
		}
		centers = append(centers, timings[index].StartMS+timings[index].DurationMS/2)
		centerValues = append(centerValues, values[index])
	}
	length := max(2, int(math.Ceil(durationMS/frameMS))+1)
	curve := make([]float64, length)
	if len(centers) == 0 {
		return &PitchContour{FrameMS: frameMS, Cents: curve}, nil
	}
	for frame := range curve {
		timeMS := float64(frame) * frameMS
		curve[frame] = interpolateManualPoint(centers, centerValues, timeMS)
	}
	return &PitchContour{FrameMS: frameMS, Cents: curve}, nil
}

func (file *ManualPitchFile) frameCurve(durationMS, frameMS float64) (*PitchContour, error) {
	if durationMS <= 0 {
		return nil, fmt.Errorf("manual pitch duration must be positive")
	}
	length := max(2, int(math.Ceil(durationMS/frameMS))+1)
	curve := make([]float64, length)
	if len(file.Frames) == 0 {
		return &PitchContour{FrameMS: frameMS, Cents: curve}, nil
	}
	if file.FrameMS > 0 {
		for index := range curve {
			position := float64(index) * frameMS / file.FrameMS
			if position >= float64(len(file.Frames)) {
				continue
			}
			left := int(math.Floor(position))
			progress := position - float64(left)
			curve[index] = file.Frames[left] * (1 - progress)
			if left+1 < len(file.Frames) {
				curve[index] += file.Frames[left+1] * progress
			}
		}
		return &PitchContour{FrameMS: frameMS, Cents: curve}, nil
	}
	if len(file.Frames) == 1 {
		for index := range curve {
			curve[index] = file.Frames[0]
		}
		return &PitchContour{FrameMS: frameMS, Cents: curve}, nil
	}
	scale := float64(len(file.Frames)-1) / float64(length-1)
	for index := range curve {
		position := float64(index) * scale
		left := int(math.Floor(position))
		if left >= len(file.Frames)-1 {
			curve[index] = file.Frames[len(file.Frames)-1]
			continue
		}
		progress := position - float64(left)
		curve[index] = file.Frames[left]*(1-progress) + file.Frames[left+1]*progress
	}
	return &PitchContour{FrameMS: frameMS, Cents: curve}, nil
}

func interpolateManualPoint(times, values []float64, timeMS float64) float64 {
	if timeMS <= times[0] {
		return values[0]
	}
	for index := 1; index < len(times); index++ {
		if timeMS <= times[index] {
			span := times[index] - times[index-1]
			if span <= 0 {
				return values[index]
			}
			progress := (timeMS - times[index-1]) / span
			return values[index-1]*(1-progress) + values[index]*progress
		}
	}
	return values[len(values)-1]
}
