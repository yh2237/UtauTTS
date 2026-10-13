package native

import "github.com/yh2237/UtauTTS/internal/tts"

func (e *Engine) SynthesisSourcePaths(data []byte) ([]string, error) {
	request, err := decodeSynthesizeRequest(data)
	if err != nil {
		return nil, err
	}
	resolved, err := e.synth.ResolveSynthesis(request.InternalRequest())
	if err != nil {
		return nil, err
	}
	return tts.SynthesisSourcePaths(resolved.Config)
}
