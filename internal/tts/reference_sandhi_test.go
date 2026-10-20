package tts

import (
	"testing"

	"github.com/yh2237/UtauTTS/internal/frontend"
)

// TestChineseSurfaceReadingは、参照TTSへ渡す読みに連続変調が入ることを確かめる。
func TestChineseSurfaceReading(t *testing.T) {
	for _, test := range []struct {
		text, want string
	}{
		// 三声が続くと前が二声になる。
		{"你好。", "ni2 hao3 |"},
		{"很好。", "hen2 hao3 |"},
		// 軽声が続くときは変調しない。
		{"我们。", "wo3 men5 |"},
	} {
		reading, morae, err := frontend.ParseChineseCVVC(test.text, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := chineseSurfaceReading(reading, morae); got != test.want {
			t.Errorf("%q: %q -> %q, want %q", test.text, reading, got, test.want)
		}
	}
}

// TestChineseSurfaceReadingKeepsMismatchは、数が合わないときに読みを変えないことを確かめる。
func TestChineseSurfaceReadingKeepsMismatch(t *testing.T) {
	const reading = "ni3 hao3"
	if got := chineseSurfaceReading(reading, nil); got != reading {
		t.Errorf("mismatched reading changed to %q", got)
	}
}
