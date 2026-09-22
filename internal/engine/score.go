package engine

type NeuralScore struct {
	Symbols   []string  `json:"symbols"`
	Durations []int64   `json:"durations"`
	F0        []float32 `json:"f0"`
	MIDI      int       `json:"midi"`
	// NoteMIDIは音符(単語)ごとのMIDI、PhMIDIは音素ごとのMIDI。話声向けにF0から求める。
	NoteMIDI []float32 `json:"note_midi,omitempty"`
	PhMIDI   []int64   `json:"ph_midi,omitempty"`
	// StepsとDurationPredictorMixはDiffSinger推論の任意調整値。0なら既定値を使う。
	Steps                int64   `json:"steps,omitempty"`
	DurationPredictorMix float32 `json:"duration_predictor_mix,omitempty"`
	// ExprはDiffSingerの表現力（既定1.0）。0なら既定値を使う。
	Expr              float32 `json:"expr,omitempty"`
	WordDiv           []int64 `json:"word_div,omitempty"`
	WordDur           []int64 `json:"word_dur,omitempty"`
	NoteRest          []bool  `json:"note_rest,omitempty"`
	UsePitchPredictor bool    `json:"use_pitch_predictor,omitempty"`
	PitchPredictorMix float32 `json:"pitch_predictor_mix,omitempty"`
}
