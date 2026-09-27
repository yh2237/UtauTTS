package voicebank

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceLibraryRejectsInvalidIntervalsAndIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.json")
	valid := `{"version":1,"language":"zh","time_origin":"oto-offset","entries":[{"source_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","duration_ms":300,"phones":[{"symbol":"e","start_ms":0,"end_ms":200},{"symbol":"n","start_ms":200,"end_ms":300}]}]}`
	if err := os.WriteFile(path, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSourcePhoneLibrary(path); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{`{"version":2,"language":"zh","time_origin":"oto-offset"}`, `{"version":1,"language":"zh","time_origin":"oto-offset","entries":[{"source_sha256":"invalid","duration_ms":300,"phones":[]}]}`, `{"version":1,"language":"zh","time_origin":"oto-offset","entries":[{"source_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","duration_ms":300,"phones":[{"symbol":"n","start_ms":200,"end_ms":400}]}]}`} {
		os.WriteFile(path, []byte(bad), 0600)
		if _, err := LoadSourcePhoneLibrary(path); err == nil {
			t.Fatal("invalid library accepted")
		}
	}
}
