package jsut

import (
	"fmt"
	"math"

	"utautts/internal/acoustic"
	"utautts/internal/audio"
)

const defaultFrameMS = 30.0

// AttachAudioFeaturesは接続監査と同じ粗い境界観測値を付加する
// 学習時だけ使うメタデータ
func AttachAudioFeatures(alignment *Alignment, pcm *audio.PCM, frameMS float64) error {
	if alignment == nil {
		return fmt.Errorf("alignment is nil")
	}
	if pcm == nil || pcm.SampleRate <= 0 || pcm.Channels <= 0 {
		return fmt.Errorf("invalid pcm")
	}
	if frameMS <= 0 || !finite(frameMS) {
		frameMS = defaultFrameMS
	}
	wave := acoustic.Mono(pcm)
	if len(wave) < 32 {
		return fmt.Errorf("audio is too short")
	}
	alignment.AudioSampleRate = pcm.SampleRate
	alignment.AudioChannels = pcm.Channels
	alignment.AudioDurationMS = float64(len(wave)) * 1000 / float64(pcm.SampleRate)
	alignment.AnalysisFrameMS = frameMS
	alignment.SpectrumBands = 10
	for index := range alignment.Phones {
		phone := &alignment.Phones[index]
		center := (phone.StartMS + phone.EndMS) / 2
		frame, ok := frameAt(wave, pcm.SampleRate, center, frameMS)
		if !ok {
			continue
		}
		jsonFrame := frameToJSON(frame)
		phone.Frame = &jsonFrame
	}
	for index := range alignment.Boundaries {
		boundary := &alignment.Boundaries[index]
		if !boundary.Trainable {
			continue
		}
		left, leftOK := frameAt(wave, pcm.SampleRate, boundary.BoundaryTimeMS-frameMS/2, frameMS)
		right, rightOK := frameAt(wave, pcm.SampleRate, boundary.BoundaryTimeMS+frameMS/2, frameMS)
		if !leftOK || !rightOK {
			continue
		}
		features := &BoundaryFeatures{
			PreviousOutgoing: frameToJSON(left),
			CurrentIncoming:  frameToJSON(right),
			SpectrumDeltaDB:  acoustic.MeanSpectrumDelta(left.SpectrumDB, right.SpectrumDB),
			RMSDeltaDB:       math.Abs(left.RMSDB - right.RMSDB),
		}
		leftVoiced, rightVoiced := left.F0Hz > 0, right.F0Hz > 0
		features.VoicingMismatch = leftVoiced != rightVoiced
		if leftVoiced && rightVoiced {
			features.F0Comparable = true
			features.F0DeltaCents = math.Abs(1200 * math.Log2(right.F0Hz/left.F0Hz))
		}
		boundary.Features = features
	}
	return nil
}

func frameAt(wave []float64, sampleRate int, centerMS, frameMS float64) (acoustic.Frame, bool) {
	if len(wave) < 32 || sampleRate <= 0 || frameMS <= 0 || !finite(centerMS) || !finite(frameMS) {
		return acoustic.Frame{}, false
	}
	halfWindow := maxInt(16, int(math.Round(frameMS*float64(sampleRate)/2000)))
	if len(wave) < halfWindow*2 {
		return acoustic.Frame{}, false
	}
	center := int(math.Round(centerMS * float64(sampleRate) / 1000))
	center = maxInt(halfWindow, minInt(len(wave)-halfWindow, center))
	values := wave[center-halfWindow : center+halfWindow]
	frame := acoustic.AnalyzeFrame(values, sampleRate, 10, true)
	return frame, frame.Valid
}

func frameToJSON(frame acoustic.Frame) Frame {
	return Frame{Valid: frame.Valid, RMSDB: frame.RMSDB, F0Hz: frame.F0Hz, SpectrumDB: append([]float64(nil), frame.SpectrumDB...)}
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
