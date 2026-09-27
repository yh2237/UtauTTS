package tts

import (
	"utautts/internal/frontend"
	"utautts/internal/prosody"
)

type SpeechTrainingPhone struct {
	Position   int      `json:"position"`
	PhoneIndex int      `json:"phone_index"`
	Symbol     string   `json:"symbol"`
	Role       string   `json:"role"`
	BaselineMS float64  `json:"baseline_ms"`
	Features   []string `json:"features"`
}

// SpeechTrainingPhonesは規則値を出力する。実測値ではない。
func SpeechTrainingPhones(morae []frontend.Mora, baseMS float64) []SpeechTrainingPhone {
	durations := speechPhoneDurations(morae, baseMS)
	features := prosody.SpeechPhoneFeatures(morae)
	var rows []SpeechTrainingPhone
	for i, m := range morae {
		if m.Pause {
			continue
		}
		for j, p := range m.Phones {
			rows = append(rows, SpeechTrainingPhone{i, j, p.Symbol, p.Role, durations[i][j], features[i][j]})
		}
	}
	return rows
}
