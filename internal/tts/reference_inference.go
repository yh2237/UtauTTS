package tts

// ReferenceReadは外で求めたKokoroの読み。AudioがnilならDurationsとF0だけ。
type ReferenceRead struct {
	Key       string
	Durations []int
	F0        []float32
	Audio     []float32
}

// ReferenceFeaturesは外で求めたWavLMの特徴量（Frames×Dims）。
type ReferenceFeatures struct {
	Key    string
	Values []float32
	Frames int
	Dims   int
}

// ProvideReferenceInferenceは、refspeech.NeedInferenceで頼まれた推論の結果を渡す（Web版）。
func ProvideReferenceInference(dir string, reads []ReferenceRead, features []ReferenceFeatures, prefetched []string) error {
	service, err := ReferenceService(dir)
	if err != nil {
		return err
	}
	for _, key := range prefetched {
		if err := service.ProvidePrefetched(key); err != nil {
			return err
		}
	}
	for _, read := range reads {
		if err := service.ProvideRead(read.Key, read.Durations, read.F0, read.Audio); err != nil {
			return err
		}
	}
	for _, item := range features {
		if err := service.ProvideFeatures(item.Key, item.Values, item.Frames, item.Dims); err != nil {
			return err
		}
	}
	return nil
}
