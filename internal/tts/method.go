package tts

import (
	"fmt"
	"sync"

	"github.com/yh2237/UtauTTS/internal/linguistics"
)

// Methodは合成の方式。方式ごとのパッケージが登録する。
type Method interface {
	// Synthesizeは、読みを解決した発話を合成する。音源はcfg.Voicebankに入っている。
	Synthesize(cfg Config, utterance linguistics.Utterance) (*Result, error)
	// PredictProsodyは、合成せずにモーラ長と抑揚を予測する。
	PredictProsody(cfg Config, utterance linguistics.Utterance) (*ProsodyPreview, error)
}

var (
	methodsMu sync.RWMutex
	methods   = map[string]Method{}
)

// RegisterMethodは方式を登録する。同じIDを2度登録するとpanicする。
func RegisterMethod(id string, method Method) {
	methodsMu.Lock()
	defer methodsMu.Unlock()
	if _, exists := methods[id]; exists {
		panic(fmt.Sprintf("tts: method %q is already registered", id))
	}
	methods[id] = method
}

func lookupMethod(id string) (Method, error) {
	if err := ValidateMethod(id); err != nil {
		return nil, err
	}
	methodsMu.RLock()
	defer methodsMu.RUnlock()
	method, ok := methods[id]
	if !ok {
		return nil, fmt.Errorf("method %q is not available in this build", id)
	}
	return method, nil
}
