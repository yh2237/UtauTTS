package worldrender

import "fmt"

// Worldは、フレーム周期とD4Cのthresholdを指定してWORLDエンジンを使う。
type World struct{ engine worldEngine }

type worldAnalyzerWith interface {
	AnalyzeWith(samples []float64, sampleRate int, inputF0 []float64, frameMS, d4cThreshold float64) (worldFeatures, error)
}

func OpenWorld(enginePath string) (*World, error) {
	engine, err := openWorldEngine(enginePath)
	if err != nil {
		return nil, err
	}
	return &World{engine: engine}, nil
}

func (w *World) Close() error { return w.engine.Close() }

// Analyzeはharvest・CheapTrick・D4Cの結果を返す。spectrumとaperiodicityはフレームごとにfftSize/2+1個並ぶ。
func (w *World) Analyze(samples []float64, sampleRate int, frameMS, d4cThreshold float64) (f0, spectrum, aperiodicity []float64, fftSize int, err error) {
	analyzer, ok := w.engine.(worldAnalyzerWith)
	if !ok {
		return nil, nil, nil, 0, fmt.Errorf("WORLD analysis options are unavailable on this platform")
	}
	features, err := analyzer.AnalyzeWith(samples, sampleRate, nil, frameMS, d4cThreshold)
	if err != nil {
		return nil, nil, nil, 0, err
	}
	return features.F0, features.Spectrum, features.Aperiodicity, features.FFTSize, nil
}

func (w *World) Synthesize(f0, spectrum, aperiodicity []float64, fftSize, sampleRate int, frameMS float64) ([]float64, error) {
	return w.engine.Synthesize(worldFeatures{Frames: len(f0), FFTSize: fftSize, F0: f0, Spectrum: spectrum,
		Aperiodicity: aperiodicity, FramePeriodMS: frameMS}, sampleRate)
}
