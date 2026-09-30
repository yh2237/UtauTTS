package audio

import "testing"

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
			got, err := TrimPCM(testPCM(1000), tc.offset, tc.cutoff)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Data) != tc.wantFrames {
				t.Fatalf("frames = %d, want %d", len(got.Data), tc.wantFrames)
			}
		})
	}
}

func testPCM(frames int) *PCM {
	data := make([]int16, frames)
	for i := range data {
		data[i] = int16(i)
	}
	return &PCM{SampleRate: 1000, Channels: 1, Data: data}
}
