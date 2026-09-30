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
