package worldrender

import "github.com/yh2237/UtauTTS/internal/audio"

// AnalyzeFileF0は、WAVファイルのF0をフレームごとに返す。enginePathが空ならruntimeから探す。
func AnalyzeFileF0(enginePath, audioPath string, frameMS float64) ([]float64, error) {
	if enginePath == "" {
		var err error
		if enginePath, err = ResolveEnginePath(); err != nil {
			return nil, err
		}
	}
	world, err := OpenWorld(enginePath)
	if err != nil {
		return nil, err
	}
	defer world.Close()
	pcm, err := audio.ReadWav(audioPath)
	if err != nil {
		return nil, err
	}
	samples := make([]float64, len(pcm.Data)/pcm.Channels)
	for i := range samples {
		for c := 0; c < pcm.Channels; c++ {
			samples[i] += float64(pcm.Data[i*pcm.Channels+c]) / 32768
		}
		samples[i] /= float64(pcm.Channels)
	}
	f0, _, _, _, err := world.Analyze(samples, pcm.SampleRate, frameMS, 0)
	return f0, err
}
