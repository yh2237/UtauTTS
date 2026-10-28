//go:build !js

package tts

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yh2237/UtauIDX/refmatch"
	"github.com/yh2237/UtauTTS/internal/oto"
	"github.com/yh2237/UtauTTS/internal/refspeech"
	"github.com/yh2237/UtauTTS/internal/voicebank"
	"github.com/yh2237/UtauWLD/worldrender"
)

func TestRealVoicebankPhonemeIndex(t *testing.T) {
	root := os.Getenv("UTAUTTS_PHONEME_VOICEBANK")
	if root == "" {
		t.Skip("UTAUTTS_PHONEME_VOICEBANK is not set")
	}
	bank, err := voicebank.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	recordings := referenceRecordings(bank, "")
	models, _, _ := ReferenceModels(os.Getenv("UTAUTTS_PHONEME_MODELS"))
	cache := os.Getenv("UTAUTTS_PHONEME_CACHE")
	if cache == "" {
		cache = t.TempDir()
	}
	world := os.Getenv("UTAUTTS_PHONEME_WORLD")
	if world == "" {
		world, err = worldrender.ResolveEnginePath()
		if err != nil {
			t.Fatal(err)
		}
	}
	service := refspeech.Shared(models, cache)
	index, err := service.Index(context.Background(), bank.Root, recordings, world)
	if err != nil {
		t.Fatal(err)
	}
	if index.Version != 8 || len(index.Phonemes) == 0 || len(index.FilePhonemes) != len(index.Files) {
		t.Fatalf("missing phoneme metadata: %+v", index.Header)
	}
	for file, id := range index.FilePhonemes {
		if id < 0 {
			continue
		}
		if int(id) >= len(index.Phonemes) {
			t.Fatalf("invalid phoneme ID %d", id)
		}
		found := false
		for _, recording := range recordings {
			if recording.Path == index.Files[file] {
				found = true
				if recording.Phoneme != index.Phonemes[id] {
					t.Fatalf("phoneme mismatch for %s", recording.Path)
				}
			}
		}
		if !found {
			t.Fatalf("unknown recording %s", index.Files[file])
		}
	}
	path := filepath.Join(cache, "phoneme-verification.idx")
	if err := index.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := refmatch.LoadIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(index.Header, loaded.Header) {
		t.Fatal("saved phoneme metadata differs")
	}
	report, err := json.MarshalIndent(loaded.Header, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "phoneme-verification.json"), report, 0600); err != nil {
		t.Fatal(err)
	}
	known := 0
	for _, id := range loaded.FilePhonemes {
		if id >= 0 {
			known++
		}
	}
	t.Logf("version=%d recordings=%d labeled=%d phonemes=%d frames=%d saved=%s", loaded.Version, len(loaded.Files), known, len(loaded.Phonemes), loaded.Frames, path)
}

func TestReferenceRecordingsCarryPhonemes(t *testing.T) {
	bank := &voicebank.Bank{Entries: map[string][]oto.Entry{
		"- あ": {{Filename: "a.wav", Alias: "- あ"}},
		"a か": {{Filename: "ka.wav", Alias: "a か"}},
	}}
	recordings := referenceRecordings(bank, "")
	if len(recordings) != 2 || recordings[0].Phoneme != "a" || recordings[1].Phoneme != "ka" {
		t.Fatalf("recordings = %+v", recordings)
	}
}
