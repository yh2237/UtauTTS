package sourceaudio

import (
	"errors"
	"math"

	"github.com/yh2237/audiodsp/wav"
)

// TrimPCMは、oto.iniの左ブランク（offsetMS）と右ブランク（blankMS）で原音を切り出す。
func TrimPCM(pcm *wav.PCM, offsetMS float64, blankMS float64) (*wav.PCM, error) {
	start, end, err := TrimFrames(pcm, offsetMS, blankMS)
	if err != nil {
		return nil, err
	}
	trimmed := make([]int16, (end-start)*pcm.Channels)
	copy(trimmed, pcm.Data[start*pcm.Channels:end*pcm.Channels])
	return &wav.PCM{SampleRate: pcm.SampleRate, Channels: pcm.Channels, Data: trimmed}, nil
}

// TrimFramesは、切り出す範囲をフレーム番号で返す。右ブランクが負なら、左ブランクからの長さとして扱う。
func TrimFrames(pcm *wav.PCM, offsetMS float64, blankMS float64) (int, int, error) {
	if pcm == nil {
		return 0, 0, errors.New("nil pcm")
	}
	if pcm.Channels <= 0 {
		return 0, 0, errors.New("invalid channel count")
	}
	frames := len(pcm.Data) / pcm.Channels
	if frames == 0 {
		return 0, 0, errors.New("empty pcm data")
	}
	start := msToFrames(offsetMS, pcm.SampleRate)
	var end int
	if blankMS < 0 {
		end = start + msToFrames(-blankMS, pcm.SampleRate)
	} else {
		end = frames - msToFrames(blankMS, pcm.SampleRate)
	}
	end = min(end, frames)
	end = max(end, 0)
	if start >= end {
		return 0, 0, errors.New("invalid trim range")
	}
	return start, end, nil
}

func msToFrames(ms float64, sampleRate int) int {
	if ms <= 0 {
		return 0
	}
	return int(math.Round(ms / 1000 * float64(sampleRate)))
}
