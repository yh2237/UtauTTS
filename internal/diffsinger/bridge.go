package diffsinger

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"utautts/internal/audio"
)

type Request struct {
	Version                   int       `json:"version"`
	AcousticPath              string    `json:"acoustic_path"`
	VocoderPath               string    `json:"vocoder_path"`
	OutputPath                string    `json:"output_path"`
	Tokens                    []int64   `json:"tokens"`
	Durations                 []int64   `json:"durations"`
	F0                        []float32 `json:"f0"`
	SampleRate                int       `json:"sample_rate"`
	Steps                     int64     `json:"steps"`
	Speedup                   int64     `json:"speedup"`
	Depth                     float32   `json:"depth,omitempty"`
	UseContinuousAcceleration bool      `json:"use_continuous_acceleration,omitempty"`
	UseVariableDepth          bool      `json:"use_variable_depth,omitempty"`
	PitchControllable         bool      `json:"pitch_controllable,omitempty"`
	Languages                 []int64   `json:"languages,omitempty"`
	UseGender                 bool      `json:"use_gender,omitempty"`
	UseVelocity               bool      `json:"use_velocity,omitempty"`
	UseEnergy                 bool      `json:"use_energy,omitempty"`
	UseBreathiness            bool      `json:"use_breathiness,omitempty"`
	UseVoicing                bool      `json:"use_voicing,omitempty"`
	UseTension                bool      `json:"use_tension,omitempty"`
	SpeakerEmbed              []float32 `json:"speaker_embed,omitempty"`
	DurationLinguisticPath    string    `json:"duration_linguistic_path,omitempty"`
	DurationPredictorPath     string    `json:"duration_predictor_path,omitempty"`
	DurationTokens            []int64   `json:"duration_tokens,omitempty"`
	DurationLanguages         []int64   `json:"duration_languages,omitempty"`
	DurationSpeakerEmbed      []float32 `json:"duration_speaker_embed,omitempty"`
	DurationPredictorMix      float32   `json:"duration_predictor_mix,omitempty"`
	WordDiv                   []int64   `json:"word_div,omitempty"`
	WordDur                   []int64   `json:"word_dur,omitempty"`
	PhMIDI                    []int64   `json:"ph_midi,omitempty"`
	PitchLinguisticPath       string    `json:"pitch_linguistic_path,omitempty"`
	PitchPredictorPath        string    `json:"pitch_predictor_path,omitempty"`
	PitchTokens               []int64   `json:"pitch_tokens,omitempty"`
	PitchLanguages            []int64   `json:"pitch_languages,omitempty"`
	PitchSpeakerEmbed         []float32 `json:"pitch_speaker_embed,omitempty"`
	PitchPredictsDur          bool      `json:"pitch_predicts_dur,omitempty"`
	PitchContinuous           bool      `json:"pitch_continuous,omitempty"`
	PitchUseExpr              bool      `json:"pitch_use_expr,omitempty"`
	PitchUseNoteRest          bool      `json:"pitch_use_note_rest,omitempty"`
	PitchPredictorMix         float32   `json:"pitch_predictor_mix,omitempty"`
	NoteMIDI                  []float32 `json:"note_midi,omitempty"`
	NoteRest                  []bool    `json:"note_rest,omitempty"`
	VarianceLinguisticPath    string    `json:"variance_linguistic_path,omitempty"`
	VariancePredictorPath     string    `json:"variance_predictor_path,omitempty"`
	VarianceTokens            []int64   `json:"variance_tokens,omitempty"`
	VarianceLanguages         []int64   `json:"variance_languages,omitempty"`
	VarianceSpeakerEmbed      []float32 `json:"variance_speaker_embed,omitempty"`
	VariancePredictsDur       bool      `json:"variance_predicts_dur,omitempty"`
	VariancePredictsEnergy    bool      `json:"variance_predicts_energy,omitempty"`
	VariancePredictsBreath    bool      `json:"variance_predicts_breathiness,omitempty"`
	VariancePredictsVoicing   bool      `json:"variance_predicts_voicing,omitempty"`
	VariancePredictsTension   bool      `json:"variance_predicts_tension,omitempty"`
	VarianceContinuous        bool      `json:"variance_continuous,omitempty"`
	MelScale                  float32   `json:"mel_scale,omitempty"`
}

func Render(ctx context.Context, bridgePath string, request Request) (*audio.PCM, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(bridgePath) == "" {
		return nil, fmt.Errorf("DiffSinger bridge is not configured")
	}
	if _, err := os.Stat(bridgePath); err != nil {
		return nil, fmt.Errorf("DiffSinger bridge %q: %w", bridgePath, err)
	}
	temp, err := os.MkdirTemp("", "utautts-diffsinger-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	request.Version = 1
	request.OutputPath = filepath.Join(temp, "output.wav")
	manifestPath := filepath.Join(temp, "request.json")
	data, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, bridgePath, manifestPath)
	output, err := command.CombinedOutput()
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, fmt.Errorf("DiffSinger bridge canceled: %w", ctxErr)
		}
		return nil, fmt.Errorf("DiffSinger bridge failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	pcm, err := audio.ReadWav(request.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("read DiffSinger output: %w", err)
	}
	return pcm, nil
}
