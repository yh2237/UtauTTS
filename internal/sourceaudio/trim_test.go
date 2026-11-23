package sourceaudio

import (
	"testing"

	"github.com/yh2237/audiodsp/wav"
)

func TestTrimPCM(t *testing.T) {
	for _, tc := range []struct {
		name           string
		offset, cutoff float64
		wantFrames     int
	}{
		{"positive cutoff", 100, 200, 700},
		{"negative cutoff is length from offset", 100, -300, 300},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := make([]int16, 1000)
			for i := range data {
				data[i] = int16(i)
			}
			got, err := TrimPCM(&wav.PCM{SampleRate: 1000, Channels: 1, Data: data}, tc.offset, tc.cutoff)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Data) != tc.wantFrames || got.Data[0] != 100 {
				t.Fatalf("frames = %d first = %d, want %d frames from 100", len(got.Data), got.Data[0], tc.wantFrames)
			}
		})
	}
}

func TestTrimPCMRejectsNilAndEmptyRange(t *testing.T) {
	if _, err := TrimPCM(nil, 0, 0); err == nil {
		t.Fatal("nil PCM accepted")
	}
	if _, err := TrimPCM(&wav.PCM{SampleRate: 1000, Channels: 1, Data: make([]int16, 10)}, 20, 0); err == nil {
		t.Fatal("offset past the end accepted")
	}
}
