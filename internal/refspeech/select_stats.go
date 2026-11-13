package refspeech

import (
	"fmt"

	"github.com/yh2237/UtauIDX/refmatch"
)

// 公開済みのUtauIDXでも合成できるよう、統計の取得は対応する版だけで行う。
func synthesizeWithSelectStats(index *refmatch.Index, world refmatch.World, request refmatch.Request, logf func(string)) ([]float64, error) {
	if observed, ok := any(index).(interface {
		SynthesizeWithStats(refmatch.World, refmatch.Request, func(frames, runs, switches, uniqueUnits int, meanRunFrames float64)) ([]float64, error)
	}); ok && logf != nil {
		return observed.SynthesizeWithStats(world, request, func(frames, runs, switches, uniqueUnits int, meanRunFrames float64) {
			logf(fmt.Sprintf("SelectStats (top1, 5ms): Frames=%d Runs=%d Switches=%d MeanRunFrames=%.6f UniqueUnits=%d",
				frames, runs, switches, meanRunFrames, uniqueUnits))
		})
	}
	return index.Synthesize(world, request)
}
