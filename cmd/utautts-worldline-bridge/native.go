package main

type nativeRequest struct {
	CacheKey          string
	SampleRate        int
	Samples           []float64
	FRQ               []byte
	Tone              int
	ConsonantVelocity float64
	OffsetMS          float64
	RequiredLengthMS  float64
	ConsonantMS       float64
	CutoffMS          float64
	Volume            float64
	Modulation        float64
	Tempo             float64
	PitchBend         []int32
	FlagG             int
	FlagO             int
	FlagP             int
	FlagMt            int
	FlagMb            int
	FlagMv            int
}

type phraseTiming struct {
	PositionMS float64
	SkipMS     float64
	LengthMS   float64
	FadeInMS   float64
	FadeOutMS  float64
}

type nativeLibrary interface {
	Close() error
	Resample(nativeRequest) ([]float32, error)
	PhraseNew() (uintptr, error)
	PhraseDelete(uintptr)
	PhraseAdd(uintptr, int, nativeRequest, phraseTiming) error
	PhraseSetCurves(uintptr, []float64, []float64, []float64, []float64, []float64) error
	PhraseSynth(uintptr) ([]float32, error)
}
