package main

import (
	"math"
	"testing"
)

func TestWorldEnvelopeUsesLinearFades(t *testing.T) {
	item := unit{LengthMS: 200, FadeInMS: 50, FadeOutMS: 50}
	if got := worldEnvelopeWeight(item, 25); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("fade-in weight = %f, want 0.5", got)
	}
}

func TestMapWorldSourceTimePreservesConsonantAndStretchesTail(t *testing.T) {
	item := unit{ConsonantMS: 100, RequiredLengthMS: 500, ConsonantVelocity: 100}
	if got := mapWorldSourceTime(item, 300, 80); got != 80 {
		t.Fatalf("consonant time = %f, want 80", got)
	}
	if got := mapWorldSourceTime(item, 300, 300); math.Abs(got-200) > 1e-9 {
		t.Fatalf("stretched tail time = %f, want 200", got)
	}
}

func TestWorldFeatureCacheIsBounded(t *testing.T) {
	cache := newWorldFeatureCache(2)
	cache.put("a", cachedWorldUnit{})
	cache.put("b", cachedWorldUnit{})
	if _, found := cache.get("a"); !found {
		t.Fatal("recent entry was not found")
	}
	cache.put("c", cachedWorldUnit{})
	if _, found := cache.get("b"); found {
		t.Fatal("least recently used entry was not evicted")
	}
}
