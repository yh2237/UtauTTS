package jsut

import "testing"

func TestParseHTSLabelsKeepsPhonesAndClassifiesBoundaries(t *testing.T) {
	text := "0 1000000 xx^xx-sil+k=a/A:0+1+1/F:1_1\n" +
		"1000000 3000000 xx^sil-k+a/A:1+1+1/F:1_1\n" +
		"3000000 5000000 sil^k-k+a/A:1+1+1/F:1_1\n" +
		"5000000 7000000 k-a+pau/A:1+1+1/F:1_1\n" +
		"7000000 9000000 a-pau+i/A:1+1+1/F:1_1\n" +
		"9000000 10000000 pau-i+sil/A:1+1+1/F:1_1\n" +
		"10000000 11000000 i-sil+xx/A:1+1+1/F:1_1"
	phones, boundaries, err := ParseHTSLabels(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(phones) != 7 || len(boundaries) != 6 {
		t.Fatalf("phones=%d boundaries=%d", len(phones), len(boundaries))
	}
	if phones[1].Symbol != "k" || phones[1].Previous != "sil" || phones[1].Next != "a" || phones[1].DurationMS != 200 || phones[1].AccentPhrasePosition != 1 || phones[1].AccentPhraseLength != 1 || phones[1].AccentNucleus != 1 {
		t.Fatalf("phone=%+v", phones[1])
	}
	if boundaries[1].JoinType != "phone" || !boundaries[1].Trainable || boundaries[1].Label == nil || *boundaries[1].Label != 1 {
		t.Fatalf("phone boundary=%+v", boundaries[1])
	}
	if boundaries[2].JoinType != "phone" || !boundaries[2].Trainable {
		t.Fatalf("second phone boundary=%+v", boundaries[2])
	}
	if boundaries[3].JoinType != "pause" || boundaries[3].Trainable {
		t.Fatalf("pause boundary=%+v", boundaries[3])
	}
	if boundaries[0].JoinType != "silence" || boundaries[5].JoinType != "silence" {
		t.Fatalf("silence boundaries=%+v %+v", boundaries[0], boundaries[5])
	}
	record := NewAlignment("BASIC5000_0001", "テスト", "sample.wav", phones, boundaries)
	if err := record.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestParseHTSLabelsRejectsNoncontiguousIntervals(t *testing.T) {
	_, _, err := ParseHTSLabels("0 100 x-sil+k=a\n101 200 x-k+sil=a")
	if err == nil {
		t.Fatal("expected noncontiguous interval error")
	}
}
