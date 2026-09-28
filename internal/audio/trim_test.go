package audio

import "testing"

func TestTrimPCMPositiveCutoff(t *testing.T) {
	pcm := testPCM(1000)
	got, err := TrimPCM(pcm, 100, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 700 {
		t.Fatalf("frames = %d, want 700", len(got.Data))
	}
}

func TestTrimPCMNegativeCutoffIsLengthFromOffset(t *testing.T) {
	pcm := testPCM(1000)
	got, err := TrimPCM(pcm, 100, -300)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 300 {
		t.Fatalf("frames = %d, want 300", len(got.Data))
	}
}

func testPCM(frames int) *PCM {
	data := make([]int16, frames)
	for i := range data {
		data[i] = int16(i)
	}
	return &PCM{SampleRate: 1000, Channels: 1, Data: data}
}
