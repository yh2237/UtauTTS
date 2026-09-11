package tts

import "testing"

func TestContextTransitionRequiresMatchingExperiment(t *testing.T) {
	cfg := Config{ProtectContextTransition: true, SpeechTiming: true, SourceContextExperiment: "recover", Renderer: "utautts-world-phrase", Language: "en", Phonemizer: "en-delta"}
	if err := validateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Config){
		func(c *Config) { c.SpeechTiming = false },
		func(c *Config) { c.SourceContextExperiment = "off" },
		func(c *Config) { c.Renderer = "waveform" },
		func(c *Config) { c.Language = "ja"; c.Phonemizer = "" },
	} {
		invalid := cfg
		change(&invalid)
		if validateConfig(invalid) == nil {
			t.Fatal("unsupported configuration accepted", invalid)
		}
	}
}
