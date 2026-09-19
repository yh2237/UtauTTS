package prosody

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"utautts/internal/frontend"
)

const ManualPitchVersion = 1

// ManualPitchFileはモデル輪郭に対する相対補正を保持する。
// mode=offset: モーラ単位の補正。mode=frames: 時刻基準の10ms単位フレーム補正。
type ManualPitchFile struct {
	Version int                `json:"version"`
	Reading string             `json:"reading,omitempty"`
	Mode    string             `json:"mode,omitempty"`
	Points  []ManualPitchPoint `json:"points,omitempty"`
	Frames  []float64          `json:"frames,omitempty"`
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
		if math.IsNaN(point.Cents) || math.IsInf(point.Cents, 0) || math.Abs(point.Cents) > 1200 {
			return fmt.Errorf("manual pitch point %d is outside +/-1200 cents", index)
		}
	}
	if file.Mode == "frames" {
		if len(file.Points) > 0 {
			return fmt.Errorf("manual pitch frames mode must not carry mora points")
		}
		if len(file.Frames) > 20000 {
			return fmt.Errorf("manual pitch frames are limited to 20000 entries")
		}
		for index, cents := range file.Frames {
			if math.IsNaN(cents) || math.IsInf(cents, 0) || math.Abs(cents) > 1200 {
				return fmt.Errorf("manual pitch frame %d is outside +/-1200 cents", index)
			}
		}
	}
	return nil
}

// Curveは疎なモーラ補正を滑らかな10msフレーム輪郭へ変換する。
// framesモードでは時刻基準の補正列を現在の長さへ伸縮して返す。
func (file *ManualPitchFile) Curve(morae []frontend.Mora, timings []MoraTiming, durationMS float64) (*PitchContour, error) {
	if file == nil {
		return nil, nil
	}
	if file.Mode == "frames" {
		return file.frameCurve(durationMS)
	}
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
	frameMS := 10.0
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

func (file *ManualPitchFile) frameCurve(durationMS float64) (*PitchContour, error) {
	frameMS := 10.0
	if durationMS <= 0 {
		return nil, fmt.Errorf("manual pitch duration must be positive")
	}
	length := max(2, int(math.Ceil(durationMS/frameMS))+1)
	curve := make([]float64, length)
	if len(file.Frames) == 0 {
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
