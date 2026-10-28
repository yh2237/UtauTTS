package refspeech

import (
	"fmt"
	"math"
	"slices"

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

func synthesizeAudio(index *refmatch.Index, world refmatch.World, features []float32, frames int, referenceF0, levels []float64, options AudioOptions) (Speech, error) {
	return synthesizeAudioAP(index, world, features, frames, referenceF0, levels, nil, options)
}

func synthesizeAudioAP(index *refmatch.Index, world refmatch.World, features []float32, frames int, referenceF0, levels []float64, referenceAP []float32, options AudioOptions) (Speech, error) {
	ratio, referenceMedian, indexMedian, err := audioPitchRatio(referenceF0, index.F0, options.PitchSemitones)
	if err != nil {
		return Speech{}, err
	}
	if options.Logf != nil {
		options.Logf(fmt.Sprintf("ピッチ補正: 参照の中央値 %.6f Hz、索引の中央値 %.6f Hz、倍率 %.9f", referenceMedian, indexMedian, ratio))
	}
	matched := index
	if options.FeatureMode != refmatch.FeatureCepstrum {
		matched, features, err = standardizeAudioFeatures(index, features, frames)
		if err != nil {
			return Speech{}, err
		}
	}
	stats := &VoicingStats{Frames: len(referenceF0), Before: voicedFrames(referenceF0)}
	if referenceAP == nil {
		refmatch.FillVoicedByLevel(referenceF0, levels, -35)
	} else {
		if len(referenceAP) != len(referenceF0) {
			return Speech{}, fmt.Errorf("reference AP and F0 do not match")
		}
		// 大きな音の子音を音量だけで有声に戻さない。APが高ければF0があっても無声にする。
		for i, ap := range referenceAP {
			if ap >= 0.5 {
				referenceF0[i] = 0
			}
		}
	}
	stats.After = voicedFrames(referenceF0)
	// OutputF0による中央値の移動とは重ねず、参照から作ったF0へ一度だけ掛ける。
	automaticF0 := slices.Clone(referenceF0)
	for i, hz := range automaticF0 {
		if hz > 0 {
			automaticF0[i] = hz * ratio
		}
	}
	request := refmatch.Request{
		FeatureMode: options.FeatureMode, TargetPhonemes: options.TargetPhonemes,
		Features: features, Frames: frames, ReferenceF0: referenceF0, OutputF0: automaticF0,
		ReferenceLevel: levels, ReferenceAP: referenceAP, ContinuityBonus: options.ContinuityBonus,
	}
	if options.FeatureMode == refmatch.FeatureCepstrum {
		request.Cepstrum, request.Features = features, nil
	}
	rendered, err := synthesizeWithSelectStats(matched, world, request, options.Logf)
	if err != nil {
		return Speech{}, err
	}
	return Speech{Audio: rendered, SampleRate: matched.SampleRate, F0: automaticF0, AutomaticF0: automaticF0, ReferenceVoicing: stats}, nil
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

func audioPitchRatio(reference []float64, bank []float32, semitones *float64) (ratio, referenceMedian, indexMedian float64, err error) {
	referenceMedian = voicedMedian(reference)
	indexMedian = voicedMedian(bank)
	ratio = 1
	if semitones != nil {
		if math.IsNaN(*semitones) || math.IsInf(*semitones, 0) {
			return 0, referenceMedian, indexMedian, fmt.Errorf("pitch semitones must be finite")
		}
		ratio = math.Exp2(math.Max(-12, math.Min(12, *semitones)) / 12)
	} else if referenceMedian > 0 && indexMedian > 0 {
		ratio = math.Max(0.5, math.Min(2, indexMedian/referenceMedian))
	}
	return ratio, referenceMedian, indexMedian, nil
}

func voicedMedian[T ~float32 | ~float64](f0 []T) float64 {
	voiced := make([]float64, 0, len(f0))
	for _, hz := range f0 {
		v := float64(hz)
		if v > 0 && !math.IsInf(v, 0) {
			voiced = append(voiced, v)
		}
	}
	if len(voiced) == 0 {
		return 0
	}
	slices.Sort(voiced)
	middle := len(voiced) / 2
	if len(voiced)%2 == 0 {
		return voiced[middle-1]/2 + voiced[middle]/2
	}
	return voiced[middle]
}

func voicedFrames(f0 []float64) int {
	count := 0
	for _, hz := range f0 {
		if hz > 0 {
			count++
		}
	}
	return count
}

// standardizeAudioFeaturesは索引の統計で両側を標準化する。元の索引とその検索キャッシュは変更しない。
func standardizeAudioFeatures(index *refmatch.Index, features []float32, frames int) (*refmatch.Index, []float32, error) {
	dims := index.Dims
	if dims <= 0 || index.Frames <= 0 || len(index.Features) != index.Frames*dims || frames <= 0 || len(features) != frames*dims {
		return nil, nil, fmt.Errorf("audio features and index dimensions do not match")
	}
	// 保存済みの索引はL2正規化済みなので、参照も同じ空間へ移してから統計を適用する。
	// 索引も再正規化し、保存時の量子化によるノルムのずれを除く。
	bank := unitFeatureRows(index.Features, dims)
	query := unitFeatureRows(features, dims)
	mean, variance := make([]float64, dims), make([]float64, dims)
	for row := range index.Frames {
		for d := range dims {
			v := float64(bank[row*dims+d])
			delta := v - mean[d]
			mean[d] += delta / float64(row+1)
			variance[d] += delta * (v - mean[d])
		}
	}
	inverseStd := make([]float64, dims)
	for d := range dims {
		inverseStd[d] = 1 / math.Max(math.Sqrt(math.Max(variance[d], 0)/float64(index.Frames)), 1e-6)
	}
	transform := func(values []float32) {
		for i, v := range values {
			d := i % dims
			values[i] = float32((float64(v) - mean[d]) * inverseStd[d])
		}
		normalizeFeatureRows(values, dims)
	}
	// nearestは参照をL2正規化して内積で比較するため、変換後の索引も単位長にする。
	transform(bank)
	transform(query)
	// sync.Onceとpacked行列はコピーしない。波形パラメータは読み取り専用で共有する。
	matched := indexWithFeatures(index, bank)
	return matched, query, nil
}

func unitFeatureRows(values []float32, dims int) []float32 {
	out := append([]float32(nil), values...)
	normalizeFeatureRows(out, dims)
	return out
}

func normalizeFeatureRows(values []float32, dims int) {
	for start := 0; start < len(values); start += dims {
		row := values[start : start+dims]
		var energy float64
		for _, v := range row {
			energy += float64(v) * float64(v)
		}
		scale := 1 / math.Max(math.Sqrt(energy), 1e-12)
		for d, v := range row {
			row[d] = float32(float64(v) * scale)
		}
	}
}
