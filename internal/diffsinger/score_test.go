package diffsinger

import (
	"reflect"
	"testing"
)

func TestRequestFromScoreBuildsProviderRequest(t *testing.T) {
	predictDur := false
	useVariableDepth := true
	singer := &Singer{
		Config: Config{
			Phonemes:                  "phonemes.txt",
			SampleRate:                44100,
			MelBase:                   "10",
			UseLangID:                 true,
			UseVariableDepth:          &useVariableDepth,
			MaxDepth:                  500,
			UseKeyShiftEmbed:          true,
			UseSpeedEmbed:             true,
			UseEnergyEmbed:            true,
			UseBreathinessEmbed:       true,
			UseVoicingEmbed:           true,
			UseTensionEmbed:           true,
			UseContinuousAcceleration: false,
		},
		Vocoder:      VocoderConfig{MelBase: "e", PitchControllable: true},
		Tokens:       map[string]int64{"SP": 1, "ja/a": 2},
		LanguageIDs:  map[string]int64{"ja": 7},
		AcousticPath: "acoustic.onnx",
		VocoderPath:  "vocoder.onnx",
		SpeakerEmbed: []float32{0.1, 0.2},
		Duration: &DurationModel{
			Config:         DurationConfig{UseLangID: true},
			Tokens:         map[string]int64{"SP": 11, "ja/a": 12},
			LanguageIDs:    map[string]int64{"ja": 8},
			LinguisticPath: "duration-linguistic.onnx",
			PredictorPath:  "duration.onnx",
			SpeakerEmbed:   []float32{0.3},
		},
		Pitch: &PitchModel{
			Config:         PitchConfig{UseLangID: true, PredictDur: &predictDur, UseExpr: true, UseNoteRest: true},
			Tokens:         map[string]int64{"SP": 21, "ja/a": 22},
			LanguageIDs:    map[string]int64{"ja": 9},
			LinguisticPath: "pitch-linguistic.onnx",
			PredictorPath:  "pitch.onnx",
			SpeakerEmbed:   []float32{0.4},
		},
		Variance: &VarianceModel{
			Config:         VarianceConfig{UseLangID: true, PredictDur: true, PredictEnergy: true, PredictBreathiness: true, PredictVoicing: true, PredictTension: true},
			Tokens:         map[string]int64{"SP": 31, "ja/a": 32},
			LanguageIDs:    map[string]int64{"ja": 10},
			LinguisticPath: "variance-linguistic.onnx",
			PredictorPath:  "variance.onnx",
			SpeakerEmbed:   []float32{0.5},
		},
	}
	score := Score{
		Symbols:           []string{"SP", "ja/a"},
		Durations:         []int64{8, 10},
		F0:                []float32{440, 441},
		MIDI:              60,
		WordDiv:           []int64{1, 1},
		WordDur:           []int64{8, 10},
		NoteRest:          []bool{true, false},
		UsePitchPredictor: true,
	}

	request, err := RequestFromScore(singer, score)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(request.Tokens, []int64{1, 2}) || !reflect.DeepEqual(request.Durations, score.Durations) || !reflect.DeepEqual(request.F0, score.F0) {
		t.Fatalf("base request = %#v", request)
	}
	if request.SampleRate != 44100 || request.Steps != 20 || request.Speedup != 50 || request.Depth != 0.5 || request.MelScale < 2.3025 || request.MelScale > 2.3026 {
		t.Fatalf("base options = %#v", request)
	}
	if !reflect.DeepEqual(request.Languages, []int64{0, 7}) || !reflect.DeepEqual(request.DurationLanguages, []int64{0, 8}) || !reflect.DeepEqual(request.PitchLanguages, []int64{0, 9}) || !reflect.DeepEqual(request.VarianceLanguages, []int64{0, 10}) {
		t.Fatalf("language ids = %#v", request)
	}
	if !reflect.DeepEqual(request.DurationTokens, []int64{11, 12}) || !reflect.DeepEqual(request.PitchTokens, []int64{21, 22}) || !reflect.DeepEqual(request.VarianceTokens, []int64{31, 32}) {
		t.Fatalf("model tokens = %#v", request)
	}
	if !reflect.DeepEqual(request.WordDiv, score.WordDiv) || !reflect.DeepEqual(request.WordDur, score.WordDur) || !reflect.DeepEqual(request.PhMIDI, []int64{60, 60}) || !reflect.DeepEqual(request.NoteMIDI, []float32{60, 60}) || !reflect.DeepEqual(request.NoteRest, score.NoteRest) {
		t.Fatalf("note metadata = %#v", request)
	}
	if request.DurationPredictorMix != 0.2 || request.PitchPredictorMix != 0.03 || request.PitchPredictsDur {
		t.Fatalf("predictor options = %#v", request)
	}
}

func TestRequestFromScoreDoesNotEnablePitchPredictorForManualF0(t *testing.T) {
	singer := &Singer{
		Config:  Config{Phonemes: "phonemes.txt", SampleRate: 44100, MelBase: "10"},
		Vocoder: VocoderConfig{MelBase: "10"},
		Tokens:  map[string]int64{"SP": 1},
		Pitch: &PitchModel{
			Tokens: map[string]int64{"SP": 2},
		},
	}
	request, err := RequestFromScore(singer, Score{Symbols: []string{"SP"}, Durations: []int64{8}, F0: []float32{440}, WordDiv: []int64{1}, WordDur: []int64{8}})
	if err != nil {
		t.Fatal(err)
	}
	if len(request.PitchTokens) != 0 || len(request.NoteMIDI) != 0 {
		t.Fatalf("manual F0 unexpectedly enabled pitch predictor: %#v", request)
	}
}

func TestScoreMelScale(t *testing.T) {
	if got := scoreMelScale("10", "e"); got < 2.3025 || got > 2.3026 {
		t.Fatalf("scale = %v", got)
	}
}
