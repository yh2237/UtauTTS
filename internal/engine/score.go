package engine

type NeuralScore struct {
	Symbols           []string  `json:"symbols"`
	Durations         []int64   `json:"durations"`
	F0                []float32 `json:"f0"`
	MIDI              int       `json:"midi"`
	WordDiv           []int64   `json:"word_div,omitempty"`
	WordDur           []int64   `json:"word_dur,omitempty"`
	NoteRest          []bool    `json:"note_rest,omitempty"`
	UsePitchPredictor bool      `json:"use_pitch_predictor,omitempty"`
}
