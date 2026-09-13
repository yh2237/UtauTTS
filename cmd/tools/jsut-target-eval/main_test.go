package main

import (
	"math"
	"testing"

	"utautts/internal/jsut"
)

func TestGroupJapanesePhones(t *testing.T) {
	phones := []jsut.Phone{
		{Symbol: "sil", Silence: true},
		{Symbol: "k"}, {Symbol: "a"},
		{Symbol: "N"},
		{Symbol: "sh"}, {Symbol: "i"},
		{Symbol: "cl"},
		{Symbol: "t"}, {Symbol: "o"},
		{Symbol: "pau", Pause: true},
		{Symbol: "m"},
	}
	groups, incomplete := groupJapanesePhones(phones)
	if incomplete != 1 {
		t.Fatalf("incomplete groups=%d, want 1", incomplete)
	}
	want := [][]string{{"k", "a"}, {"N"}, {"sh", "i"}, {"cl"}, {"t", "o"}}
	if len(groups) != len(want) {
		t.Fatalf("groups=%d, want %d", len(groups), len(want))
	}
	for index, group := range groups {
		if len(group) != len(want[index]) {
			t.Fatalf("group %d length=%d, want %d", index, len(group), len(want[index]))
		}
		for phoneIndex, phone := range group {
			if phone.Symbol != want[index][phoneIndex] {
				t.Fatalf("group %d phone %d=%q, want %q", index, phoneIndex, phone.Symbol, want[index][phoneIndex])
			}
		}
	}
}

func TestNormalizeDurations(t *testing.T) {
	values := []float64{1, 2, 3}
	normalizeDurations(values, 120)
	if !almostEqual(values[0], 20) || !almostEqual(values[1], 40) || !almostEqual(values[2], 60) {
		t.Fatalf("normalized=%v", values)
	}
	zero := []float64{0, 0}
	normalizeDurations(zero, 120)
	if zero[0] != 0 || zero[1] != 0 {
		t.Fatalf("zero values changed=%v", zero)
	}
}

func almostEqual(left, right float64) bool {
	return math.Abs(left-right) < 1e-9
}
