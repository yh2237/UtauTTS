package openutau

import (
	"math"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestToneToMIDI(t *testing.T) {
	cases := []struct {
		tone string
		want int
		err  bool
	}{
		{"C4", 60, false},
		{"B3", 59, false},
		{"C2", 36, false},
		{"C5", 72, false},
		{"F#5", 78, false},
		{"Gb4", 66, false},
		{"Bb3", 58, false},
		{"A0", 21, false},
		{"60", 60, false},
		{"", 0, true},
		{"H4", 0, true},
		{"C", 0, true},
	}
	for _, tc := range cases {
		got, err := ToneToMIDI(tc.tone)
		if tc.err {
			if err == nil {
				t.Errorf("ToneToMIDI(%q) expected error, got %d", tc.tone, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ToneToMIDI(%q) unexpected error: %v", tc.tone, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ToneToMIDI(%q) = %d, want %d", tc.tone, got, tc.want)
		}
	}
}

func TestParseUtauTTSProject(t *testing.T) {
	data := []byte(`{
		"format": "utautts-project",
		"format_version": 5,
		"utterances": [{
			"text": "こんにちは",
			"voicebank_id": "熵尾音-真声",
			"tone": "C4",
			"mora_duration_ms": 140,
			"pause_duration_ms": 180,
			"intonation": 0,
			"apply_pitch": false,
			"analysis_cache": {
				"reading": "コンニチワ",
				"morae": [
					{"position": 0, "mora": "こ", "pause": false, "consonant": "k", "vowel": "o"},
					{"position": 1, "mora": "ん", "pause": false, "vowel": "n"},
					{"position": 2, "mora": "に", "pause": false, "vowel": "i"}
				]
			}
		}]
	}`)
	project, err := ParseUtauTTSProject(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(project.Utterances) != 1 {
		t.Fatalf("utterances = %d, want 1", len(project.Utterances))
	}

	if _, err := ParseUtauTTSProject([]byte(`{"format":"other","format_version":1,"utterances":[]}`)); err == nil {
		t.Error("expected error for wrong format")
	}
	if _, err := ParseUtauTTSProject([]byte(`{"format":"utautts-project","format_version":1,"utterances":[{"tone":"H9","analysis_cache":{}}]}`)); err == nil {
		t.Error("expected error for invalid tone")
	}
}

func TestExportUSTX(t *testing.T) {
	project := &UtauTTSProject{
		Format:        utauTTSProjectFormat,
		FormatVersion: 5,
		Utterances: []UtauTTSUtterance{
			{
				Text:              "みなさん、こんにちは",
				VoicebankID:       "熵尾音-真声",
				Tone:              "C4",
				MoraDurationMS:    140,
				PauseDurationMS:   180,
				ManualPitchEdited: true,
				PitchPoints:       []float64{0, 50, -30, 60, -20, 0, 10},
				AnalysisCache: UtauTTSAnalysisCache{
					Reading: "ミナサン、コンニチワ",
					Morae: []UtauTTSMora{
						{Position: 0, Mora: "み", Vowel: "i"},
						{Position: 1, Mora: "な", Vowel: "a"},
						{Position: 2, Mora: "さ", Vowel: "a"},
						{Position: 3, Mora: "ん", Vowel: "n"},
						{Position: 4, Pause: true},
						{Position: 5, Mora: "こ", Vowel: "o"},
						{Position: 6, Mora: "ん", Vowel: "n"},
						{Position: 7, Mora: "に", Vowel: "i"},
						{Position: 8, Mora: "ち", Vowel: "i"},
						{Position: 9, Mora: "は", Vowel: "a"},
					},
				},
			},
			{
				Text:                 "おはよう",
				VoicebankID:          "熵尾音-假声",
				Tone:                 "B3",
				MoraDurationMS:       150,
				PauseDurationMS:      200,
				AutomaticPitchPoints: []float64{100, 50, -25},
				AnalysisCache: UtauTTSAnalysisCache{
					Reading: "オハヨー",
					Morae: []UtauTTSMora{
						{Position: 0, Mora: "お", Vowel: "o"},
						{Position: 1, Mora: "は", Vowel: "a"},
						{Position: 2, Mora: "よ", Vowel: "o"},
					},
				},
			},
		},
	}

	data, err := ExportUSTX(project, ExportOptions{ProjectName: "テスト", BPM: 120})
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)

	for _, want := range []string{
		"ustx_version: \"0.6\"",
		"resolution: 480",
		"bpm: 120",
		"track_name: 熵尾音-真声",
		"track_name: 熵尾音-假声",
		"voice_parts:",
		"lyric: み",
		"tone: 60",
		"tone: 59",
		"snap_first: false",
		"\"y\": 5",
		"\"y\": 10",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("export missing %q\n%s", want, text)
		}
	}

	// YAMLとして再解析できることを確認する。
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("exported YAML does not parse: %v", err)
	}
	if parsed["ustx_version"] != "0.6" {
		t.Errorf("ustx_version = %v", parsed["ustx_version"])
	}
	parts := parsed["voice_parts"].([]any)
	if len(parts) != 2 {
		t.Fatalf("voice_parts = %d, want 2", len(parts))
	}
	firstPart := parts[0].(map[string]any)
	notes := firstPart["notes"].([]any)
	if len(notes) != 9 {
		t.Fatalf("first part notes = %d, want 9 (pause skipped)", len(notes))
	}
	firstNote := notes[0].(map[string]any)
	if firstNote["lyric"] != "み" || firstNote["tone"] != 60 {
		t.Errorf("first note = %v", firstNote)
	}
	pitch := firstNote["pitch"].(map[string]any)
	points := pitch["data"].([]any)
	if len(points) != 2 {
		t.Fatalf("pitch points = %d, want 2", len(points))
	}
	firstPoint := points[0].(map[string]any)
	if y := firstPoint["y"]; y != 0 && y != float64(0) {
		t.Errorf("first pitch point y = %v, want 0", y)
	}
	secondNote := notes[1].(map[string]any)
	pitch2 := secondNote["pitch"].(map[string]any)
	pts2 := pitch2["data"].([]any)
	if y := pts2[0].(map[string]any)["y"]; y != float64(5) && y != 5 {
		t.Errorf("second note pitch y = %v, want 5 (50 cents / 10)", y)
	}

	// 休止前後のトラック位置を確認する。
	if pos := firstPart["track_no"]; pos != 0 {
		t.Errorf("first part track_no = %v, want 0", pos)
	}
	secondPart := parts[1].(map[string]any)
	if secondPart["track_no"] != 1 {
		t.Errorf("second part track_no = %v, want 1", secondPart["track_no"])
	}
}

func TestExportUSTXSkipsEmptyUtterances(t *testing.T) {
	project := &UtauTTSProject{
		Format: utauTTSProjectFormat, FormatVersion: 5,
		Utterances: []UtauTTSUtterance{
			{Text: "未解析卡片", VoicebankID: "vb", Tone: "C4", AnalysisCache: UtauTTSAnalysisCache{}}, // no morae
			{
				Text: "こんにちは", VoicebankID: "vb", Tone: "C4",
				MoraDurationMS: 140, PauseDurationMS: 180,
				AnalysisCache: UtauTTSAnalysisCache{
					Morae: []UtauTTSMora{{Position: 0, Mora: "こ", Vowel: "o"}},
				},
			},
		},
	}
	data, err := ExportUSTX(project, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	parts := parsed["voice_parts"].([]any)
	if len(parts) != 1 {
		t.Fatalf("voice_parts = %d, want 1 (empty card skipped)", len(parts))
	}

	allEmpty := &UtauTTSProject{Format: utauTTSProjectFormat, FormatVersion: 5,
		Utterances: []UtauTTSUtterance{{Text: "空", VoicebankID: "vb", Tone: "C4", AnalysisCache: UtauTTSAnalysisCache{}}}}
	if _, err := ExportUSTX(allEmpty, ExportOptions{}); err == nil {
		t.Error("expected error when no utterance has notes")
	}
}

func TestExportUSTXFrameCurveSampling(t *testing.T) {
	project := &UtauTTSProject{
		Format: utauTTSProjectFormat, FormatVersion: 5,
		Utterances: []UtauTTSUtterance{{
			Text: "あい", VoicebankID: "vb", Tone: "C4",
			MoraDurationMS: 100, PauseDurationMS: 200,
			AnalysisCache: UtauTTSAnalysisCache{
				Morae: []UtauTTSMora{
					{Position: 0, Mora: "あ", Vowel: "a"},
					{Position: 1, Mora: "い", Vowel: "i"},
				},
			},
		}},
	}
	// 10ms contour: あ spans 0-100ms (cents 0..50), い spans 100-200ms (50..0).
	cents := make([]float64, 21)
	for i := range cents {
		if i <= 10 {
			cents[i] = float64(i) * 5 // 0..50
		} else {
			cents[i] = float64(20-i) * 5 // 50..0
		}
	}
	data, err := ExportUSTX(project, ExportOptions{Curves: []FrameCurve{{FrameMS: 10, Cents: cents}}})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	part := parsed["voice_parts"].([]any)[0].(map[string]any)
	notes := part["notes"].([]any)
	if len(notes) != 2 {
		t.Fatalf("notes = %d, want 2", len(notes))
	}
	first := notes[0].(map[string]any)["pitch"].(map[string]any)["data"].([]any)
	if len(first) < 10 {
		t.Fatalf("first note pitch points = %d, want ~10 (10ms sampling over 100ms)", len(first))
	}
	// 輪郭の先頭と末尾が正しく変換されることを確認する。
	firstPoint := first[0].(map[string]any)
	if x := firstPoint["x"]; x != 0 && x != float64(0) {
		t.Errorf("first point x = %v, want 0", x)
	}
	if y := firstPoint["y"]; y != 0 && y != float64(0) {
		t.Errorf("first point y = %v, want 0", y)
	}
	// ノート中央の値が補間されることを確認する。
	found := false
	for _, p := range first {
		pt := p.(map[string]any)
		if pt["x"] == float64(50) || pt["x"] == 50 {
			if y := pt["y"]; y != 2.5 && y != float64(2.5) {
				t.Errorf("point at x=50 y = %v, want 2.5", y)
			}
			found = true
		}
	}
	if !found {
		t.Error("no pitch point at x=50 in first note")
	}
}

func TestExportUSTXSequentialSameTrackParts(t *testing.T) {
	project := &UtauTTSProject{
		Format: utauTTSProjectFormat, FormatVersion: 5,
		Utterances: []UtauTTSUtterance{
			{
				Text: "おはよう", VoicebankID: "熵尾音-真声", Tone: "C4",
				MoraDurationMS: 140, PauseDurationMS: 180,
				AnalysisCache: UtauTTSAnalysisCache{
					Morae: []UtauTTSMora{
						{Position: 0, Mora: "お", Vowel: "o"},
						{Position: 1, Mora: "は", Vowel: "a"},
						{Position: 2, Mora: "よ", Vowel: "o"},
					},
				},
			},
			{
				Text: "こんにちは", VoicebankID: "熵尾音-真声", Tone: "C4",
				MoraDurationMS: 140, PauseDurationMS: 180,
				AnalysisCache: UtauTTSAnalysisCache{
					Morae: []UtauTTSMora{
						{Position: 0, Mora: "こ", Vowel: "o"},
						{Position: 1, Mora: "ん", Vowel: "n"},
						{Position: 2, Mora: "に", Vowel: "i"},
						{Position: 3, Mora: "ち", Vowel: "i"},
						{Position: 4, Mora: "は", Vowel: "a"},
					},
				},
			},
		},
	}
	data, err := ExportUSTX(project, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	parts := parsed["voice_parts"].([]any)
	if len(parts) != 2 {
		t.Fatalf("voice_parts = %d, want 2", len(parts))
	}
	// 同じトラックのパートが重ならず、1拍分の間隔を持つことを確認する。
	first := parts[0].(map[string]any)
	second := parts[1].(map[string]any)
	firstStart := first["position"].(int)
	secondStart := second["position"].(int)
	if first["track_no"] != second["track_no"] {
		t.Fatalf("parts should share one track: %v vs %v", first["track_no"], second["track_no"])
	}
	if secondStart <= firstStart {
		t.Fatalf("second part position %d must follow first part position %d (no overlap)", secondStart, firstStart)
	}
	if secondStart < 480 {
		t.Fatalf("expected at least a one-beat gap, second part starts at %d", secondStart)
	}
}

func TestExportUSTXLongVowelExtension(t *testing.T) {
	project := &UtauTTSProject{
		Format: utauTTSProjectFormat, FormatVersion: 5,
		Utterances: []UtauTTSUtterance{{
			Text: "おはよう", VoicebankID: "vb", Tone: "C4",
			MoraDurationMS: 100, PauseDurationMS: 200,
			AnalysisCache: UtauTTSAnalysisCache{
				Morae: []UtauTTSMora{
					{Position: 0, Mora: "お", Vowel: "o"},
					{Position: 1, Mora: "は", Vowel: "a"},
					{Position: 2, Mora: "よ", Vowel: "o"},
					{Position: 3, Mora: "ー", Vowel: "o"},
					{Position: 4, Mora: "ご", Vowel: "o"},
					{Position: 5, Mora: "ざ", Vowel: "a"},
					{Position: 6, Mora: "い", Vowel: "i"},
					{Position: 7, Mora: "ま", Vowel: "a"},
					{Position: 8, Mora: "す", Vowel: "u"},
				},
			},
		}},
	}
	data, err := ExportUSTX(project, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]any
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	notes := parsed["voice_parts"].([]any)[0].(map[string]any)["notes"].([]any)
	lyrics := make([]string, len(notes))
	for i, n := range notes {
		lyrics[i] = n.(map[string]any)["lyric"].(string)
	}
	// よの後の長音が「+お」の拡張ノートになることを確認する。
	want := []string{"お", "は", "よ", "+お", "ご", "ざ", "い", "ま", "す"}
	for i := range want {
		if lyrics[i] != want[i] {
			t.Errorf("note %d lyric = %q, want %q (all: %v)", i, lyrics[i], want[i], lyrics)
		}
	}
}

func TestExportUSTXDurationsAndPauses(t *testing.T) {
	project := &UtauTTSProject{
		Format: utauTTSProjectFormat, FormatVersion: 5,
		Utterances: []UtauTTSUtterance{{
			Text: "あ、い", VoicebankID: "vb", Tone: "C4",
			MoraDurationMS: 100, PauseDurationMS: 200,
			ManualMoraDurEdited: true,
			MoraDurationsMS:     []float64{80, 0, 120},
			AnalysisCache: UtauTTSAnalysisCache{
				Morae: []UtauTTSMora{
					{Position: 0, Mora: "あ", Vowel: "a"},
					{Position: 1, Pause: true},
					{Position: 2, Mora: "い", Vowel: "i"},
				},
			},
		}},
	}
	data, err := ExportUSTX(project, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	// BPM120のmsからtickへの変換を確認する。
	text := string(data)
	for _, want := range []string{"position: 0", "duration: 77", "position: 269", "duration: 115"} {
		if !strings.Contains(text, want) {
			t.Errorf("export missing %q\n%s", want, text)
		}
	}
}

func TestExportUSTXUsesAutomaticMoraTiming(t *testing.T) {
	project := &UtauTTSProject{
		Format:        "utautts-project",
		FormatVersion: 5,
		Utterances: []UtauTTSUtterance{{
			Text: "カキ", VoicebankID: "bank", Tone: "C4",
			MoraDurationMS:  140,
			PauseDurationMS: 180,
			// 固定の140msグリッドとは異なる計画上の時間。
			AutomaticMoraDurMS: []float64{100, 120},
			AutomaticMoraPosMS: []float64{0, 130},
			AnalysisCache: UtauTTSAnalysisCache{
				Reading: "カキ",
				Morae: []UtauTTSMora{
					{Position: 0, Mora: "か", Vowel: "a"},
					{Position: 1, Mora: "き", Vowel: "i"},
				},
			},
		}},
	}
	data, err := ExportUSTX(project, ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		VoiceParts []struct {
			Notes []struct {
				Position int    `yaml:"position"`
				Duration int    `yaml:"duration"`
				Lyric    string `yaml:"lyric"`
			} `yaml:"notes"`
		} `yaml:"voice_parts"`
	}
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	notes := parsed.VoiceParts[0].Notes
	if len(notes) != 2 {
		t.Fatalf("notes = %d, want 2", len(notes))
	}
	// 出力側と同じmsからtickへの変換。
	ticks := func(ms float64) int {
		return int(math.Round(ms * 480.0 / (60000.0 / 120.0)))
	}
	want := []struct {
		lyric    string
		position int
		duration int
	}{
		{"か", ticks(0), ticks(100)},
		{"き", ticks(130), ticks(120)},
	}
	for index, expectation := range want {
		note := notes[index]
		if note.Lyric != expectation.lyric || note.Position != expectation.position || note.Duration != expectation.duration {
			t.Fatalf("note[%d] = {lyric:%s position:%d duration:%d}, want {lyric:%s position:%d duration:%d}",
				index, note.Lyric, note.Position, note.Duration, expectation.lyric, expectation.position, expectation.duration)
		}
	}
}
