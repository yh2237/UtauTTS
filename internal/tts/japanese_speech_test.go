package tts

import (
	"github.com/yh2237/UtauTTS/internal/prosody"
)

func japaneseSpeechTestPredictions(count int) []prosody.Prediction {
	predictions := make([]prosody.Prediction, count)
	for i := range predictions {
		predictions[i] = prosody.Prediction{DurationFactor: 1, PitchFactor: 1, EnergyFactor: 1}
	}
	return predictions
}
