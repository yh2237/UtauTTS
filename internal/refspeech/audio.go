package refspeech

import (
	"fmt"
	"math"

	"github.com/yh2237/UtauIDX/refmatch"
)

// AudioOptionsは参照音声からの合成に使う設定。テキスト経路には適用しない。
type AudioOptions struct {
	// FeatureModeはwavlm（空なら既定）またはcepstrum。
	FeatureMode refmatch.FeatureMode
	// UseReferenceAPは参照APによる有声判定と混合を有効にする。falseなら従来どおり。
	UseReferenceAP bool
	// ContinuityBonusは同じ録音の次の位置を優先する強さ。0なら従来どおり。
	ContinuityBonus float64
	// PitchSemitonesは参照から移す半音数。nilなら音源の音域に自動で合わせる。
	// 自動・手動とも倍率を0.5〜2に制限する。
	PitchSemitones *float64
	// TargetPhonemesは20msごとの索引音素ID。nilなら音素で絞らない。
	TargetPhonemes []int32
	// PhonemeFallbackは指定した音素に近い単位が無いとき制約を外す。
	PhonemeFallback bool
	Logf            func(string)
}

// referenceAPRowsはWORLDの線形APを合成の5ms行に合わせる。
// 高域は母音でも非周期的なので、1500Hz以下の平均を有声判定と混合に使う。
func referenceAPRows(ap []float64, rate, fftSize int, frameMS float64, rows int) ([]float32, error) {
	bins := fftSize/2 + 1
	if rate <= 0 || fftSize <= 0 || frameMS <= 0 || len(ap) == 0 || len(ap)%bins != 0 || rows <= 0 {
		return nil, fmt.Errorf("invalid reference AP dimensions")
	}
	sourceRows := len(ap) / bins
	lowBins := min(bins, int(1500*float64(fftSize)/float64(rate))+1)
	means := make([]float64, sourceRows)
	for row := range sourceRows {
		for _, value := range ap[row*bins : row*bins+lowBins] {
			if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
				return nil, fmt.Errorf("invalid reference AP value")
			}
			means[row] += value / float64(lowBins)
		}
	}
	out := make([]float32, rows)
	for row := range rows {
		position := math.Min(float64(sourceRows-1), float64(row)*refmatch.WorldFrameMS()/frameMS)
		left := int(position)
		fraction := position - float64(left)
		out[row] = float32(means[left]*(1-fraction) + means[min(left+1, sourceRows-1)]*fraction)
	}
	return out, nil
}
