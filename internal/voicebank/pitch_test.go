package voicebank

import (
	"math"
	"path/filepath"
	"testing"

	"utautts/internal/audio"
	"utautts/internal/oto"
)

func TestMeasureCandidateF0(t *testing.T) {
	t.Run("vowel region", testMeasureCandidateF0UsesVowelRegion)
	t.Run("silence invalid", testMeasureCandidateF0MarksSilenceInvalid)
}

func testMeasureCandidateF0UsesVowelRegion(t *testing.T) {
	const sampleRate = 16000
	data := make([]int16, sampleRate/4)
	for index := sampleRate * 85 / 1000; index < len(data); index++ {
		data[index] = int16(7000 * math.Sin(2*math.Pi*200*float64(index)/sampleRate))
	}
	path := filepath.Join(t.TempDir(), "candidate.wav")
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: data}); err != nil {
		t.Fatal(err)
	}
	measured := measureCandidateF0(oto.Entry{Filename: path, Fixed: 80}, map[candidatePitchKey]candidatePitch{})
	if !measured.Valid || math.Abs(measured.Hz-200) > 5 {
		t.Fatalf("candidate pitch = %+v", measured)
	}
}

func testMeasureCandidateF0MarksSilenceInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "silence.wav")
	if err := audio.WriteWav(path, &audio.PCM{SampleRate: 16000, Channels: 1, Data: make([]int16, 4000)}); err != nil {
		t.Fatal(err)
	}
	measured := measureCandidateF0(oto.Entry{Filename: path}, map[candidatePitchKey]candidatePitch{})
	if measured.Valid || measured.Hz != 0 {
		t.Fatalf("silent candidate pitch = %+v", measured)
	}
}
