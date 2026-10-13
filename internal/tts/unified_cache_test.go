package tts

import (
	"context"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/render"
)

func TestUnifiedProsodyCacheSharesPreviewAndEnergyWithoutMutation(t *testing.T) {
	model, morae, features, synthesisPlan := unifiedProsodyFixture(t)
	scope := lifecycle.New()
	t.Cleanup(func() { _ = scope.Close() })
	ctx := scope.Bind(context.Background())
	timings := moraTimings(morae, synthesisPlan)
	duration := synthesisPlan.DurationMS + 20
	preview := predictUnifiedProsody(ctx, model.F0Head, "ja", features, timings, duration, synthesisPlan, false, false)
	full := predictUnifiedProsody(ctx, model.F0Head, "ja", features, timings, duration, synthesisPlan, false, true)
	if preview.contour == nil || !reflect.DeepEqual(preview.contour, full.contour) || len(full.energy) == 0 {
		t.Fatal("preview and synthesis predictions differ")
	}
	cache, err := cacheFor(ctx)
	if err != nil || len(cache.unified) != 1 {
		t.Fatalf("cache entries=%d, err=%v", len(cache.unified), err)
	}
	ids, cont, _, err := unifiedProsodyContext(model.F0Head, "ja", features, timings, duration, synthesisPlan, false)
	if err != nil {
		t.Fatal(err)
	}
	f0, energy, err := model.F0Head.PredictProsody(ids, cont)
	if err != nil {
		t.Fatal(err)
	}
	for i, value := range f0 {
		if full.contour.Cents[i] != float64(value)*(.3*1200/math.Ln2) {
			t.Fatalf("F0 differs at frame %d", i)
		}
	}
	if !reflect.DeepEqual(full.energy, energy) {
		t.Fatal("energy differs")
	}
	full.applyEnergy(model.F0Head, synthesisPlan)
	full.contour.Cents[0] = 999
	again := predictUnifiedProsody(ctx, model.F0Head, "ja", features, timings, duration, synthesisPlan, false, true)
	if !reflect.DeepEqual(again.energy, energy) || !reflect.DeepEqual(again.contour, preview.contour) {
		t.Fatal("caller mutated cached prediction")
	}
	features[0]["accent_high"] = 1
	predictUnifiedProsody(ctx, model.F0Head, "ja", features, timings, duration, synthesisPlan, false, true)
	if len(cache.unified) != 2 {
		t.Fatal("changed accent reused prediction")
	}
	copyOfModel := *model.F0Head
	predictUnifiedProsody(ctx, &copyOfModel, "ja", features, timings, duration, synthesisPlan, false, true)
	if len(cache.unified) != 3 {
		t.Fatal("changed model reused prediction")
	}
	cache.Lock()
	cache.forgetUnifiedModelLocked(model.F0Head)
	cache.Unlock()
	if len(cache.unified) != 1 {
		t.Fatal("old model predictions retained")
	}
	cache.clear()
	if len(cache.unified) != 0 || cache.unifiedBytes != 0 {
		t.Fatal("cache not cleared")
	}
}

func TestUnifiedProsodyCacheKeyTracksTimingAndMask(t *testing.T) {
	model, morae, features, synthesisPlan := unifiedProsodyFixture(t)
	timings := moraTimings(morae, synthesisPlan)
	ids, cont, speech, err := unifiedProsodyContext(model.F0Head, "ja", features, timings, 680, synthesisPlan, false)
	if err != nil {
		t.Fatal(err)
	}
	key := unifiedProsodyKey(model.F0Head, ids, cont, speech)
	cont[0][1] += .1
	if key == unifiedProsodyKey(model.F0Head, ids, cont, speech) {
		t.Fatal("length not included")
	}
	cont[0][1] -= .1
	key = unifiedProsodyKey(model.F0Head, ids, cont, speech)
	ids[0][0]++
	if key == unifiedProsodyKey(model.F0Head, ids, cont, speech) {
		t.Fatal("phone not included")
	}
	ids[0][0]--
	speech[0] = !speech[0]
	if key == unifiedProsodyKey(model.F0Head, ids, cont, speech) {
		t.Fatal("mask not included")
	}
}

func TestUnifiedProsodyCacheLimits(t *testing.T) {
	for _, frames := range []int{1, 200000} {
		cache := newSynthesisCache()
		prediction := unifiedProsodyPrediction{contour: &render.PitchCurve{Cents: make([]float64, frames)}}
		for i := 0; i <= maxProsodyCacheEntries; i++ {
			key := unifiedProsodyCacheKey{inputs: [32]byte{byte(i)}}
			cache.storeUnified(key, prediction)
		}
		if len(cache.unified) > maxProsodyCacheEntries || cache.unifiedBytes > maxUnifiedProsodyCacheBytes {
			t.Fatal("cache exceeds limit")
		}
		if _, ok := cache.unified[unifiedProsodyCacheKey{}]; ok {
			t.Fatal("oldest entry retained")
		}
	}
}

func TestUnifiedProsodyCacheConcurrentPredictions(t *testing.T) {
	model, morae, features, synthesisPlan := unifiedProsodyFixture(t)
	scope := lifecycle.New()
	t.Cleanup(func() { _ = scope.Close() })
	ctx := scope.Bind(context.Background())
	timings := moraTimings(morae, synthesisPlan)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			prediction := predictUnifiedProsody(ctx, model.F0Head, "ja", features, timings, 680, synthesisPlan, false, true)
			if prediction.contour == nil {
				t.Error("prediction failed")
				return
			}
			prediction.contour.Cents[0]++
			prediction.energy[0]++
		}()
	}
	wg.Wait()
	cache, err := cacheFor(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.unified) != 1 {
		t.Fatalf("entries=%d", len(cache.unified))
	}
}
