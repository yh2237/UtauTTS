package pitch

import dsppitch "github.com/yh2237/audiodsp/pitch"

type Detector = dsppitch.Detector

func EstimateMedian(wave []float64, sampleRate int) float64 {
	return dsppitch.EstimateMedian(wave, sampleRate)
}

func Estimate(frame []float64, sampleRate int) float64 {
	return dsppitch.Estimate(frame, sampleRate)
}
