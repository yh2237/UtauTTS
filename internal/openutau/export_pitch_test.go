package openutau

import (
	"math"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExportUSTXSavedFramePitchAndEdits(t *testing.T) {
	project, err := ParseUtauTTSProject([]byte(`{
		"format":"utautts-project","format_version":8,
		"utterances":[{
			"text":"あい","voicebank_id":"bank","tone":"C4","apply_pitch":true,
			"manual_pitch_edited":true,"pitch_points":[900,900],
			"pitch_frames":[0,20,0],
			"automatic_frame_ms":10,"automatic_frame_pitch":[0,100,0,100,0,100,0,100],
			"automatic_mora_durations_ms":[25,25],"automatic_mora_positions_ms":[5,35],
			"analysis_cache":{"reading":"アイ","morae":[{"mora":"あ"},{"mora":"い"}]}
		}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	// 保存済みの曲線は、出力時に再予測した曲線より優先する。
	data, err := ExportUSTX(project, ExportOptions{Curves: []FrameCurve{{FrameMS: 10, Cents: []float64{999, 999}}}})
	if err != nil {
		t.Fatal(err)
	}
	var got USTXProject
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	notes := got.VoiceParts[0].Notes
	want := [][]USTXPitchPoint{
		{{X: 0, Y: 6}, {X: 5, Y: 12}, {X: 15, Y: 0}, {X: 25, Y: 10}},
		{{X: 0, Y: 5}, {X: 5, Y: 0}, {X: 15, Y: 10}, {X: 25, Y: 0}},
	}
	if len(notes) != len(want) {
		t.Fatalf("notes = %d, want %d", len(notes), len(want))
	}
	for i, note := range notes {
		if note.Pitch.SnapFirst || len(note.Pitch.Data) != len(want[i]) {
			t.Fatalf("note %d pitch = %+v", i, note.Pitch)
		}
		for j, point := range note.Pitch.Data {
			if math.Abs(point.X-want[i][j].X) > 1e-9 || math.Abs(point.Y-want[i][j].Y) > 1e-9 || point.Shape != "l" {
				t.Fatalf("note %d point %d = %+v, want %+v with linear interpolation", i, j, point, want[i][j])
			}
		}
	}
}

func TestExportUSTXManualFrameFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		manual bool
		frames []float64
		points []float64
		wantY  float64
	}{
		{"frame correction without automatic frames", true, []float64{20}, []float64{900}, 7},
		{"mora correction without automatic frames", true, nil, []float64{20}, 7},
		{"zero frames use mora correction", true, []float64{0, 0}, []float64{20}, 7},
		{"disabled edits", false, []float64{20}, []float64{900}, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := &UtauTTSProject{Utterances: []UtauTTSUtterance{{
				VoicebankID: "bank", Tone: "C4", MoraDurationMS: 100,
				ManualPitchEdited: tc.manual, PitchFrames: tc.frames, PitchPoints: tc.points,
				AutomaticPitchPoints: []float64{50},
				AnalysisCache:        UtauTTSAnalysisCache{Morae: []UtauTTSMora{{Mora: "あ"}}},
			}}}
			data, err := ExportUSTX(project, ExportOptions{})
			if err != nil {
				t.Fatal(err)
			}
			var got USTXProject
			if err := yaml.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if y := got.VoiceParts[0].Notes[0].Pitch.Data[0].Y; y != tc.wantY {
				t.Fatalf("first point y = %v, want %v", y, tc.wantY)
			}
		})
	}
}
