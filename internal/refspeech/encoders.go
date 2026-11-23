package refspeech

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"strings"

	"github.com/yh2237/UtauIDX/encoder"
	"github.com/yh2237/UtauIDX/index"
	"github.com/yh2237/UtauIDX/matcher"
	"github.com/yh2237/gograd/dsp"
)

func envPath(path string, keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return path
}

func encoderEnabled(id string) bool {
	filter, ok := os.LookupEnv("UTAUTTS_ENCODERS")
	if !ok {
		return true
	}
	for _, value := range strings.Split(filter, ",") {
		if strings.TrimSpace(value) == id {
			return true
		}
	}
	return false
}

func loadEncoders(models Models) (_ *encoder.Registry, err error) {
	reg := encoder.NewRegistry()
	loaded := false
	defer func() {
		if !loaded {
			err = errors.Join(err, reg.Close())
		}
	}()
	for _, model := range []struct {
		id       string
		path     string
		optional bool
		load     func(string) (encoder.Encoder, error)
	}{
		{encoder.IDWavLMBasePlusL6, envPath(models.WavLM, "UTAUTTS_WAVLM_PATH", "UTAUTTS_HUBERT_PATH"), false, func(path string) (encoder.Encoder, error) {
			return encoder.LoadWav2Vec2(path, encoder.Wav2Vec2Options{ID: encoder.IDWavLMBasePlusL6, Layer: 6})
		}},
		{encoder.IDWav2Vec2EspeakL20, envPath(models.Espeak, "UTAUTTS_ESPEAK_PATH"), true, func(path string) (encoder.Encoder, error) {
			return encoder.LoadWav2Vec2(path, encoder.Wav2Vec2Options{
				ID: encoder.IDWav2Vec2EspeakL20, Layer: 20,
				ConfigPath:       envPath(models.EspeakConfig, "UTAUTTS_ESPEAK_CONFIG"),
				PreprocessorPath: envPath(models.EspeakPreprocessor, "UTAUTTS_ESPEAK_PREPROCESSOR"),
			})
		}},
		{encoder.IDWhisperSmallL12PosInv, envPath(models.Whisper, "UTAUTTS_WHISPER_PATH"), true, func(path string) (encoder.Encoder, error) {
			return encoder.LoadWhisperEncoder(path, encoder.WhisperOptions{
				ID: encoder.IDWhisperSmallL12PosInv, Layer: 12, ConfigPath: envPath(models.WhisperConfig, "UTAUTTS_WHISPER_CONFIG"),
			})
		}},
	} {
		if !encoderEnabled(model.id) {
			continue
		}
		if model.optional {
			info, err := os.Stat(model.path)
			if model.path == "" || os.IsNotExist(err) || (err == nil && !info.Mode().IsRegular()) {
				continue
			}
			if err != nil {
				return nil, err
			}
		}
		enc, err := model.load(model.path)
		if err != nil {
			return nil, fmt.Errorf("load reference encoder %s: %w", model.id, err)
		}
		reg.Register(enc)
	}
	if len(reg.IDs()) == 0 {
		return nil, errors.New("no reference encoders enabled")
	}
	loaded = true
	return reg, nil
}

func (e *Engine) queryFeatures(idx *index.Index, samples []float64, rate int, key string) (map[string][]float32, map[string]int, int, error) {
	var cached *cachedQueryFeatures
	if key != "" {
		cached = e.cachedFeatures(key)
	}
	var targets []index.ChannelMeta
	var extractors []encoder.Encoder
	for _, channel := range idx.Header.Channels {
		if channel.ID == matcher.ChannelMouthStatic || channel.ID == matcher.ChannelMouthDelta || channel.Dims <= 0 {
			continue
		}
		if e.registry != nil {
			enc := e.registry.Get(channel.ID)
			if enc != nil && enc.Dims() == channel.Dims {
				targets = append(targets, channel)
				extractors = append(extractors, enc)
			}
		} else if cached != nil && cached.dims[channel.ID] == channel.Dims {
			targets = append(targets, channel)
		}
	}
	if len(targets) == 0 {
		return nil, nil, 0, errors.New("no shared neural channels between the reference and the index")
	}

	channels := make(map[string][]float32, len(targets))
	dims := make(map[string]int, len(targets))
	for _, channel := range targets {
		dims[channel.ID] = channel.Dims
		if cached != nil && cached.dims[channel.ID] == channel.Dims {
			channels[channel.ID] = cached.channels[channel.ID]
		}
	}
	extractMissing := len(channels) < len(targets)
	if extractMissing {
		SetStage("特徴量を計算しています")
		wave := make([]float32, len(samples))
		for i, sample := range samples {
			wave[i] = float32(sample)
		}
		wave = dsp.Resample(wave, rate, encoder.SampleRate)
		var extractErr error
		extract := func() {
			for _, enc := range extractors {
				id, width := enc.ID(), enc.Dims()
				if _, ok := channels[id]; ok {
					continue
				}
				values, frames, err := enc.Extract(wave)
				if err != nil {
					extractErr = fmt.Errorf("extract %s: %w", id, err)
					return
				}
				if frames == 0 {
					extractErr = fmt.Errorf("audio is too short for %s", id)
					return
				}
				if frames < 0 || frames > math.MaxInt/width || len(values) != frames*width {
					extractErr = fmt.Errorf("invalid feature shape for %s", id)
					return
				}
				channels[id] = values
			}
		}
		if e.externalStore != nil {
			jobs := e.externalStore.Collect(extract)
			if extractErr == nil && len(jobs) > 0 {
				return nil, nil, 0, wrapNeedInference(&encoder.NeedInference{Jobs: jobs})
			}
		} else {
			extract()
		}
		if extractErr != nil {
			return nil, nil, 0, wrapNeedInference(extractErr)
		}
	}

	var frames int
	if !extractMissing && cached != nil && len(cached.channels) == len(channels) {
		frames = cached.frames
	} else {
		frames = commonFrames(channels, dims)
	}
	if frames <= 0 {
		return nil, nil, 0, errors.New("reference has no feature frames")
	}
	if key != "" && extractMissing {
		all := make(map[string][]float32)
		widths := make(map[string]int)
		if cached != nil {
			maps.Copy(all, cached.channels)
			maps.Copy(widths, cached.dims)
		}
		maps.Copy(all, channels)
		maps.Copy(widths, dims)
		// Keep the original frame counts for indexes with different channels.
		e.storeFeatures(key, all, commonFrames(all, widths), widths)
	}
	aligned := make(map[string][]float32, len(channels))
	for id, values := range channels {
		aligned[id] = values[:frames*dims[id]]
	}
	return aligned, dims, frames, nil
}

func commonFrames(channels map[string][]float32, dims map[string]int) int {
	frames := math.MaxInt
	for id, values := range channels {
		frames = min(frames, len(values)/dims[id])
	}
	return frames
}
