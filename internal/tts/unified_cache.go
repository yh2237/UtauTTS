package tts

import (
	"crypto/sha256"
	"encoding/binary"
	"math"

	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauWLD/speechtiming"
)

const maxUnifiedProsodyCacheBytes = 16 << 20

type unifiedProsodyCacheKey struct {
	model  *speechtiming.TCN
	inputs [32]byte
}

// 強さや手動補正ではなく、推論器へ渡す入力で一致を判定する。
func unifiedProsodyKey(model *speechtiming.TCN, ids [][3]int, cont [][]float32, speech []bool) unifiedProsodyCacheKey {
	h := sha256.New()
	row := make([]byte, (3+model.F0Context())*4+1)
	for t, phones := range ids {
		for i, id := range phones {
			binary.LittleEndian.PutUint32(row[i*4:], uint32(id))
		}
		for i, value := range cont[t] {
			binary.LittleEndian.PutUint32(row[(i+3)*4:], math.Float32bits(value))
		}
		row[len(row)-1] = 0
		if speech[t] {
			row[len(row)-1] = 1
		}
		_, _ = h.Write(row)
	}
	return unifiedProsodyCacheKey{model: model, inputs: [32]byte(h.Sum(nil))}
}

func (p unifiedProsodyPrediction) bytes() int {
	return len(p.contour.Cents)*8 + len(p.energy)*4 + len(p.ids)*3*8
}

func (p unifiedProsodyPrediction) copy(includeEnergy bool) unifiedProsodyPrediction {
	p.contour = &render.PitchCurve{FrameMS: p.contour.FrameMS, Cents: append([]float64(nil), p.contour.Cents...)}
	if includeEnergy {
		p.energy = append([]float32(nil), p.energy...)
	} else {
		p.energy = nil
	}
	return p
}

func (cache *synthesisCacheState) storeUnified(key unifiedProsodyCacheKey, value unifiedProsodyPrediction) {
	size := value.bytes()
	if size > maxUnifiedProsodyCacheBytes {
		return
	}
	cache.Lock()
	defer cache.Unlock()
	if _, ok := cache.unified[key]; ok {
		return
	}
	cache.unified[key] = value
	cache.unifiedOrder = append(cache.unifiedOrder, key)
	cache.unifiedBytes += size
	for len(cache.unifiedOrder) > maxProsodyCacheEntries || cache.unifiedBytes > maxUnifiedProsodyCacheBytes {
		oldest := cache.unifiedOrder[0]
		copy(cache.unifiedOrder, cache.unifiedOrder[1:])
		cache.unifiedOrder[len(cache.unifiedOrder)-1] = unifiedProsodyCacheKey{}
		cache.unifiedOrder = cache.unifiedOrder[:len(cache.unifiedOrder)-1]
		cache.unifiedBytes -= cache.unified[oldest].bytes()
		delete(cache.unified, oldest)
	}
}

func (cache *synthesisCacheState) forgetUnifiedModelLocked(model *speechtiming.TCN) {
	order := cache.unifiedOrder[:0]
	for _, key := range cache.unifiedOrder {
		if key.model == model {
			cache.unifiedBytes -= cache.unified[key].bytes()
			delete(cache.unified, key)
		} else {
			order = append(order, key)
		}
	}
	clear(cache.unifiedOrder[len(order):])
	cache.unifiedOrder = order
}
