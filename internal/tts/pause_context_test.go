package tts

import "testing"

func TestPauseContextEnabledDefaultsOn(t *testing.T) {
	if !pauseContextEnabled(Config{}) {
		t.Fatal("pause context was disabled by default")
	}
	disabled := false
	if pauseContextEnabled(Config{PauseContext: &disabled}) {
		t.Fatal("explicit false did not disable pause context")
	}
	enabled := true
	if !pauseContextEnabled(Config{PauseContext: &enabled}) {
		t.Fatal("explicit true did not enable pause context")
	}
}

func TestPauseContextStrengthDefaults(t *testing.T) {
	if got := pauseContextStrength(Config{}); got != 1 {
		t.Fatalf("default strength = %v, want 1", got)
	}
	if got := pauseContextStrength(Config{PauseContextStrength: 0.5}); got != 0.5 {
		t.Fatalf("explicit strength = %v, want 0.5", got)
	}
	if got := pauseContextStrength(Config{PauseContextStrength: -1}); got != -1 {
		t.Fatalf("negative strength = %v, want -1", got)
	}
}
