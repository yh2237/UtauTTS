package tts

import (
	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

func SynthesisSourcePaths(cfg Config) ([]string, error) {
	if scope := lifecycle.FromContext(cfg.Context); scope != nil {
		ctx, release, err := scope.Begin(cfg.Context)
		if err != nil {
			return nil, err
		}
		defer release()
		cfg.Context = ctx
	}
	if err := synthesisContextError(cfg.Context); err != nil {
		return nil, err
	}
	bank := cfg.Voicebank
	if bank == nil {
		var err error
		bank, err = loadVoicebankCachedContext(cfg.Context, cfg.VoicebankPath)
		if err != nil {
			return nil, err
		}
	}
	cfg.Voicebank = bank
	applyAliasProfile(bank, &cfg)
	applyVoicebankDictionary(bank, &cfg)
	_, _, _, morae, err := resolvePronunciation(cfg)
	if err != nil {
		return nil, err
	}
	return bank.CandidateSourcePaths(morae, voicebank.ResolveConfig{Tone: cfg.Tone, Color: cfg.Color, AliasPolicy: cfg.AliasPolicy}), nil
}
