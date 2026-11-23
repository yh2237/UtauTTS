package tts

import "testing"

func TestStretchAdaptEnabledRequiresLongTargetMora(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
		want bool
	}{
		{"short mora is disabled", Config{StretchAdapt: true, MoraDurationMS: 140}, false},
		{"unset mora is disabled", Config{StretchAdapt: true}, false},
		{"base mora at threshold is enabled", Config{StretchAdapt: true, MoraDurationMS: stretchAdaptMinMoraMS}, true},
		{"long position mora is enabled", Config{StretchAdapt: true, MoraDurationMS: 140, MoraDurationsMS: []float64{140, 400, 140}}, true},
		{"false overrides long mora", Config{StretchAdapt: false, MoraDurationMS: 400}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := stretchAdaptEnabled(tc.cfg); got != tc.want {
				t.Fatalf("stretchAdaptEnabled(%+v) = %t, want %t", tc.cfg, got, tc.want)
			}
		})
	}
}
