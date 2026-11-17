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
		{{X: 0, Y: 96}, {X: 5, Y: 102}, {X: 15, Y: 90}, {X: 25, Y: 100}},
		{{X: 0, Y: 95}, {X: 5, Y: 90}, {X: 15, Y: 100}, {X: 25, Y: 90}},
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

func TestExportUSTXManualFrameSampleTimes(t *testing.T) {
	for _, tc := range []struct {
		name    string
		frameMS float64
		want    []USTXPitchPoint
	}{
		{"default 10ms", 0, []USTXPitchPoint{{X: 0, Y: 8}, {X: 10, Y: 16}, {X: 20, Y: 8}}},
		{"5ms short edit", 5, []USTXPitchPoint{{X: 0, Y: 8}, {X: 5, Y: 16}, {X: 10, Y: 8}}},
		{"20ms interpolated samples", 20, []USTXPitchPoint{{X: 0, Y: 8}, {X: 10, Y: 12}, {X: 20, Y: 16}, {X: 30, Y: 12}, {X: 40, Y: 8}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, duration := range []float64{100, 200} {
				utterance := UtauTTSUtterance{
					VoicebankID: "bank", Tone: "C4", MoraDurationMS: duration,
					ManualPitchEdited: true, PitchFrames: []float64{0, 80, 0}, PitchPoints: []float64{30},
					AutomaticFrameMS: tc.frameMS, AutomaticPitchPoints: []float64{50},
					AnalysisCache: UtauTTSAnalysisCache{Morae: []UtauTTSMora{{Mora: "あ"}}},
				}
				note := exportPitchNotes(t, utterance, ExportOptions{})[0]
				for _, want := range tc.want {
					assertExportPitchPoint(t, note, want.X, want.Y)
				}
				assertExportPitchPoint(t, note, duration, 8)
			}
		})
	}
}

func TestExportUSTXManualPointsInterpolateMoraCenters(t *testing.T) {
	for _, tc := range []struct {
		name               string
		manualTiming       bool
		automaticPositions []float64
		wantPositions      []float64
		wantDurations      []float64
		want               [][]USTXPitchPoint
	}{
		{
			name:          "automatic durations and pause",
			wantPositions: []float64{0, 130, 250}, wantDurations: []float64{80, 120, 100},
			want: [][]USTXPitchPoint{
				{{X: 0, Y: 10}, {X: 40, Y: 10}, {X: 80, Y: 10 + 20*40.0/150}},
				{{X: 0, Y: 22}, {X: 60, Y: 30}, {X: 120, Y: 30 * 50.0 / 110}},
				{{X: 0, Y: 30 * 50.0 / 110}, {X: 50, Y: 0}, {X: 100, Y: 0}},
			},
		},
		{
			name: "manual timing overrides automatic timing", manualTiming: true,
			automaticPositions: []float64{0, 900, 900, 900},
			wantPositions:      []float64{20, 200, 330}, wantDurations: []float64{100, 100, 100},
			want: [][]USTXPitchPoint{
				{{X: 0, Y: 10}, {X: 50, Y: 10}, {X: 100, Y: 10 + 20*50.0/180}},
				{{X: 0, Y: 10 + 20*130.0/180}, {X: 50, Y: 30}, {X: 100, Y: 30 * 80.0 / 130}},
				{{X: 0, Y: 30 * 50.0 / 130}, {X: 50, Y: 0}, {X: 100, Y: 0}},
			},
		},
		{
			name: "partial positions keep pauses from advancing the cursor", automaticPositions: []float64{20},
			wantPositions: []float64{20, 100, 220}, wantDurations: []float64{80, 120, 100},
			want: [][]USTXPitchPoint{
				{{X: 0, Y: 10}, {X: 40, Y: 10}, {X: 80, Y: 18}},
				{{X: 0, Y: 18}, {X: 60, Y: 30}, {X: 120, Y: 30 * 50.0 / 110}},
				{{X: 0, Y: 30 * 50.0 / 110}, {X: 50, Y: 0}, {X: 100, Y: 0}},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			utterance := UtauTTSUtterance{
				VoicebankID: "bank", Tone: "C4", MoraDurationMS: 100, PauseDurationMS: 50,
				ManualPitchEdited: true, PitchPoints: []float64{100, 999, 300},
				ManualMoraDurEdited: tc.manualTiming,
				MoraDurationsMS:     []float64{100, 999, 100, 100}, MoraPositionsMS: []float64{20, 900, 200, 330},
				AutomaticMoraDurMS: []float64{80, 999, 120, 100}, AutomaticMoraPosMS: tc.automaticPositions,
				AnalysisCache: UtauTTSAnalysisCache{Morae: []UtauTTSMora{{Mora: "あ"}, {Pause: true}, {Mora: "い"}, {Mora: "う"}}},
			}
			notes := exportPitchNotes(t, utterance, ExportOptions{BPM: 120})
			if len(notes) != len(tc.want) {
				t.Fatalf("notes = %d, want %d", len(notes), len(tc.want))
			}
			for index, note := range notes {
				position := int(math.Round(tc.wantPositions[index] * 0.96))
				duration := int(math.Round(tc.wantDurations[index] * 0.96))
				if note.Position != position || note.Duration != duration {
					t.Fatalf("note %d timing = (%d, %d), want (%d, %d)", index, note.Position, note.Duration, position, duration)
				}
				for _, want := range tc.want[index] {
					assertExportPitchPoint(t, note, want.X, want.Y)
				}
			}
		})
	}
}

func TestExportUSTXUneditedPitchKeepsAutomaticCurve(t *testing.T) {
	utterance := UtauTTSUtterance{
		VoicebankID: "bank", Tone: "C4", MoraDurationMS: 20,
		PitchFrames: []float64{900, 900}, PitchPoints: []float64{900},
		AutomaticFrameMS: 5, AutomaticFramePitch: []float64{0, 80, 0}, AutomaticPitchPoints: []float64{999},
		AnalysisCache: UtauTTSAnalysisCache{Morae: []UtauTTSMora{{Mora: "あ"}}},
	}
	note := exportPitchNotes(t, utterance, ExportOptions{Curves: []FrameCurve{{FrameMS: 10, Cents: []float64{999}}}})[0]
	if len(note.Pitch.Data) != 5 {
		t.Fatalf("pitch points = %+v, want five automatic samples", note.Pitch.Data)
	}
	for _, want := range []USTXPitchPoint{{X: 0, Y: 0}, {X: 5, Y: 8}, {X: 10, Y: 0}, {X: 20, Y: 0}} {
		assertExportPitchPoint(t, note, want.X, want.Y)
	}
}

func TestExportUSTXPointsOnlyKeepTenMillisecondSampling(t *testing.T) {
	for _, frames := range [][]float64{nil, {0, 0}, {0.1, -0.1}} {
		utterance := UtauTTSUtterance{
			VoicebankID: "bank", Tone: "C4", MoraDurationMS: 20,
			ManualPitchEdited: true, PitchFrames: frames, PitchPoints: []float64{30},
			AutomaticFrameMS: 5, AutomaticPitchPoints: []float64{50},
			AnalysisCache: UtauTTSAnalysisCache{Morae: []UtauTTSMora{{Mora: "あ"}}},
		}
		note := exportPitchNotes(t, utterance, ExportOptions{})[0]
		if len(note.Pitch.Data) != 3 {
			t.Fatalf("frames=%v pitch = %+v, want three 10ms samples", frames, note.Pitch.Data)
		}
		for _, x := range []float64{0, 10, 20} {
			assertExportPitchPoint(t, note, x, 8)
		}
	}
}

func TestExportUSTXRejectsInvalidManualPitch(t *testing.T) {
	for _, tc := range []struct {
		name    string
		frameMS float64
		frames  []float64
	}{
		{"sub-millisecond frame step", 0.5, []float64{20}},
		{"frame outside correction range", 10, []float64{4801}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := &UtauTTSProject{Utterances: []UtauTTSUtterance{{
				VoicebankID: "bank", Tone: "C4", MoraDurationMS: 100,
				ManualPitchEdited: true, AutomaticFrameMS: tc.frameMS, PitchFrames: tc.frames,
				AnalysisCache: UtauTTSAnalysisCache{Morae: []UtauTTSMora{{Mora: "あ"}}},
			}}}
			if _, err := ExportUSTX(project, ExportOptions{}); err == nil {
				t.Fatal("expected invalid manual pitch to be rejected")
			}
		})
	}
}

func exportPitchNotes(t *testing.T, utterance UtauTTSUtterance, opts ExportOptions) []USTXNote {
	t.Helper()
	data, err := ExportUSTX(&UtauTTSProject{Utterances: []UtauTTSUtterance{utterance}}, opts)
	if err != nil {
		t.Fatal(err)
	}
	var got USTXProject
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	return got.VoiceParts[0].Notes
}

func assertExportPitchPoint(t *testing.T, note USTXNote, x, y float64) {
	t.Helper()
	if note.Pitch.SnapFirst {
		t.Fatal("pitch snap_first must be false")
	}
	for _, point := range note.Pitch.Data {
		if math.Abs(point.X-x) < 1e-9 {
			if math.Abs(point.Y-y) > 1e-9 || point.Shape != "l" {
				t.Fatalf("pitch at %gms = %+v, want y=%g and linear interpolation", x, point, y)
			}
			return
		}
	}
	t.Fatalf("no pitch point at %gms in %+v", x, note.Pitch.Data)
}

func TestExportUSTXManualFrameFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name   string
		manual bool
		frames []float64
		points []float64
		wantY  float64
	}{
		{"frame and mora corrections without automatic frames", true, []float64{20}, []float64{900}, 97},
		{"mora correction without automatic frames", true, nil, []float64{20}, 7},
		{"zero frames use mora correction", true, []float64{0, 0}, []float64{20}, 7},
		{"small frames use mora correction", true, []float64{0.05}, []float64{20}, 7},
		{"no edits", true, nil, nil, 5},
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
			if y := got.VoiceParts[0].Notes[0].Pitch.Data[0].Y; math.Abs(y-tc.wantY) > 1e-9 {
				t.Fatalf("first point y = %v, want %v", y, tc.wantY)
			}
		})
	}
}
