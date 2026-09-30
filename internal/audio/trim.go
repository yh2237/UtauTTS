package audio

import (
	"errors"
	"math"
)

// TrimPCMはoto.iniのoffsetとblankから録音の利用可能領域を返す。
// 子音境界は領域内の構造を示す値なので切り出しには使わない。
func TrimPCM(pcm *PCM, offsetMs float64, blankMs float64) (*PCM, error) {
	start, end, err := TrimFrames(pcm, offsetMs, blankMs)
	if err != nil {
		return nil, err
	}
	startIndex := start * pcm.Channels
	endIndex := end * pcm.Channels
	trimmed := make([]int16, endIndex-startIndex)
	copy(trimmed, pcm.Data[startIndex:endIndex])
	return &PCM{SampleRate: pcm.SampleRate, Channels: pcm.Channels, Data: trimmed}, nil
}

// TrimFramesはコピーせず、otoのoffset/blankに対応するフレーム範囲を返す。
func TrimFrames(pcm *PCM, offsetMs float64, blankMs float64) (int, int, error) {
	if pcm.Channels <= 0 {
		return 0, 0, errors.New("invalid channel count")
	}
	frames := len(pcm.Data) / pcm.Channels
	if frames == 0 {
		return 0, 0, errors.New("empty pcm data")
	}

	start := msToFrames(offsetMs, pcm.SampleRate)
	if start < 0 {
		start = 0
	}
	var end int
	if blankMs < 0 {
		end = start + msToFrames(-blankMs, pcm.SampleRate)
	} else {
		end = frames - msToFrames(blankMs, pcm.SampleRate)
	}
	if end > frames {
		end = frames
	}
	if end < 0 {
		end = 0
	}
	if start >= end {
		return 0, 0, errors.New("invalid trim range")
	}
	return start, end, nil
}

func msToFrames(ms float64, sampleRate int) int {
	if ms <= 0 {
		return 0
	}
	return int(math.Round((ms / 1000.0) * float64(sampleRate)))
}
