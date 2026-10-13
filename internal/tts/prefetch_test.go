package tts

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/oto"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func TestSourcePrefetchUsesVoicebankDictionaryWithoutMutatingInput(t *testing.T) {
	bank := &voicebank.Bank{ARPAsing: map[string]string{"cat": "K AE T", "dog": "D AO G"}, Entries: map[string][]oto.Entry{}}
	dictionary := map[string]string{"cat": "K AH T"}
	cfg := Config{Dictionary: dictionary}
	applyVoicebankDictionary(bank, &cfg)
	if cfg.Dictionary["cat"] != "K AH T" || cfg.Dictionary["dog"] != "D AO G" {
		t.Fatal("voicebank dictionary or override missing")
	}
	if !reflect.DeepEqual(dictionary, map[string]string{"cat": "K AH T"}) || bank.ARPAsing["cat"] != "K AE T" {
		t.Fatal("input dictionary changed")
	}
	bank.Entries["あ"] = []oto.Entry{{Filename: "a.wav"}}
	paths, err := SynthesisSourcePaths(Config{Voicebank: bank, Reading: "あ", Language: "ja"})
	if err != nil || !reflect.DeepEqual(paths, []string{"a.wav"}) {
		t.Fatalf("paths=%v, err=%v", paths, err)
	}
}

func TestSourcePrefetchRespectsClosedRuntime(t *testing.T) {
	scope := lifecycle.New()
	if err := scope.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := SynthesisSourcePaths(Config{Context: scope.Bind(context.Background())})
	if !errors.Is(err, lifecycle.ErrClosed) {
		t.Fatalf("err=%v", err)
	}
}
