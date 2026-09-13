package jsut

import (
	"math"
	"testing"

	"utautts/internal/audio"
)

func TestAttachAudioFeaturesMeasuresPhoneCentersAndBoundaries(t *testing.T) {
	labels := "0 1000000 xx^xx-sil+k=a/A:0+1+1/F:1_1\n" +
		"1000000 3000000 xx^sil-k+a/A:1+1+1/F:1_1\n" +
		"3000000 5000000 sil^k-k+a/A:1+1+1/F:1_1\n" +
		"5000000 7000000 k-a+pau/A:1+1+1/F:1_1\n" +
		"7000000 9000000 a-pau+i/A:1+1+1/F:1_1\n" +
		"9000000 10000000 pau-i+sil/A:1+1+1/F:1_1\n" +
		"10000000 11000000 i-sil+xx/A:1+1+1/F:1_1"
	phones, boundaries, err := ParseHTSLabels(labels)
	if err != nil {
		t.Fatal(err)
	}
	record := NewAlignment("test", "テスト", "test.wav", phones, boundaries)
	const sampleRate = 16000
	pcm := &audio.PCM{SampleRate: sampleRate, Channels: 1, Data: make([]int16, sampleRate)}
	for index := range pcm.Data {
		pcm.Data[index] = int16(12000 * math.Sin(2*math.Pi*220*float64(index)/sampleRate))
	}
	if err := AttachAudioFeatures(&record, pcm, 30); err != nil {
		t.Fatal(err)
	}
	if record.AudioSampleRate != sampleRate || record.AudioChannels != 1 || record.AudioDurationMS != 1000 || record.AnalysisFrameMS != 30 || record.SpectrumBands != 10 {
		t.Fatalf("audio metadata=%+v", record)
	}
	if record.Phones[2].Frame == nil || !record.Phones[2].Frame.Valid {
		t.Fatalf("phone frame=%+v", record.Phones[2].Frame)
	}
	if record.Boundaries[1].Features == nil || !record.Boundaries[1].Features.PreviousOutgoing.Valid || !record.Boundaries[1].Features.F0Comparable {
		t.Fatalf("boundary features=%+v", record.Boundaries[1].Features)
	}
	if record.Boundaries[0].Features != nil {
		t.Fatalf("silence boundary should not have features=%+v", record.Boundaries[0].Features)
	}
}
