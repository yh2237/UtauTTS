package tts

import "testing"

func boolPtr(value bool) *bool { return &value }

func TestStretchAdaptEnabledRequiresLongTargetMora(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{"default short mora is disabled", Config{MoraDurationMS: 140}, false},
		{"unset mora falls back to short default", Config{}, false},
		{"long base mora is enabled", Config{MoraDurationMS: 400}, true},
		{"base mora at threshold is enabled", Config{MoraDurationMS: stretchAdaptMinMoraMS}, true},
		{"long position mora is enabled", Config{MoraDurationMS: 140, MoraDurationsMS: []float64{140, 400, 140}}, true},
		{"explicit true with short mora stays disabled", Config{StretchAdapt: boolPtr(true), MoraDurationMS: 140}, false},
		{"explicit true with long mora is enabled", Config{StretchAdapt: boolPtr(true), MoraDurationMS: 400}, true},
		{"explicit false overrides long mora", Config{StretchAdapt: boolPtr(false), MoraDurationMS: 400}, false},
		{"explicit false overrides position mora", Config{StretchAdapt: boolPtr(false), MoraDurationMS: 140, MoraDurationsMS: []float64{400}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stretchAdaptEnabled(tc.cfg); got != tc.want {
				t.Fatalf("stretchAdaptEnabled(%+v) = %t, want %t", tc.cfg, got, tc.want)
			}
		})
	}
}

func TestStretchAdaptStrengths(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want float64
	}{
		{"zero uses default", Config{StretchAdaptStrength: 0}, 1},
		{"explicit value is kept", Config{StretchAdaptStrength: 0.5}, 0.5},
		{"negative is identity", Config{StretchAdaptStrength: -1}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stretchAdaptStrength(tc.cfg); got != tc.want {
				t.Fatalf("stretchAdaptStrength(%+v) = %v, want %v", tc.cfg, got, tc.want)
			}
		})
	}
}
