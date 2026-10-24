package api

import "testing"

func TestBatchFileNameKeepsOnlyTheFileName(t *testing.T) {
	for name, want := range map[string]string{
		"first.wav":        "first.wav",
		"dir/second.wav":   "second.wav",
		`..\evil.wav`:      "evil.wav",
		`C:\Windows\x.wav`: "x.wav",
		"C:drive.wav":      "",
		"..":               "",
		"a/..":             "",
		"nul\x00byte.wav":  "",
		"":                 "",
	} {
		if got := batchFileName(name); got != want {
			t.Errorf("batchFileName(%q) = %q, want %q", name, got, want)
		}
	}
}
