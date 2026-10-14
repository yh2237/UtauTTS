package prosody

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

func BenchmarkPredictFrameContour(b *testing.B) {
	model, err := LoadModel(filepath.Join("..", "..", "models", "frame-intonation-tcn-v12.json"))
	if err != nil {
		b.Fatal(err)
	}
	for _, count := range []int{100, 500} {
		b.Run(fmt.Sprintf("%d-morae", count), func(b *testing.B) {
			morae := make([]frontend.Mora, count)
			timings := make([]MoraTiming, count)
			for i := range morae {
				morae[i] = frontend.Mora{Text: "あ", Vowel: "a"}
				timings[i] = MoraTiming{StartMS: float64(i) * 120, DurationMS: 120}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if curve := model.PredictFrameContour(morae, nil, timings, float64(count)*120, false); curve == nil {
					b.Fatal("no contour")
				}
			}
		})
	}
}
