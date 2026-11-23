package refspeech

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/yh2237/UtauIDX/encoder"
	"github.com/yh2237/UtauIDX/speechpool"
	"github.com/yh2237/UtauTTS/internal/kokoro"
)

// NeedInferenceは、KokoroやWavLMの推論を外（Web版のONNX Runtime Web）に頼む必要があることを表す。
// 呼び出し側はJobsの結果をProvideReadとProvideFeaturesで渡し、同じ呼び出しをやり直す。
type NeedInference struct {
	Jobs []InferenceJob
}

func (n *NeedInference) Error() string {
	return fmt.Sprintf("reference inference is needed (%d jobs)", len(n.Jobs))
}

// InferenceJobは外に頼む推論の1件。
type InferenceJob struct {
	// Kindはkokoro（読み）、features（音声の特徴量）、files（索引を作る前に録音をまとめて取得する）。
	Kind      string `json:"kind"`
	Key       string `json:"key"`
	EncoderID string `json:"encoder_id,omitempty"`
	// Modelは重みのファイル名（拡張子なし）、Voiceは声のファイル名（拡張子なし）、Styleは声の表の行。
	Model  string  `json:"model,omitempty"`
	Voice  string  `json:"voice,omitempty"`
	Style  int     `json:"style,omitempty"`
	Tokens []int   `json:"tokens,omitempty"`
	Speed  float32 `json:"speed,omitempty"`
	// Durationsが空なら長さも予測する。Audioが偽なら長さとF0だけでよい。
	Durations []int `json:"durations,omitempty"`
	Audio     bool  `json:"audio,omitempty"`
	// Samplesはfeaturesの16 kHzの音声。
	Samples []float32 `json:"-"`
	// Pathsはfilesで取得する録音。
	Paths []string `json:"paths,omitempty"`
}

// openExternalは重みを読まずに、推論の入力を作るための設定だけを読む。
func openExternal(models Models) (*Engine, error) {
	voices := map[string]*voiceSet{}
	load := func(id, weights, config, voice string) error {
		model, err := kokoro.LoadConfig(config)
		if err != nil {
			return fmt.Errorf("load Kokoro config (%s): %w", id, err)
		}
		voices[id] = &voiceSet{model: model, modelName: stem(weights), voiceName: stem(voice)}
		return nil
	}
	if err := load(kokoroEnglish, models.KokoroWeights, models.KokoroConfig, models.KokoroVoice); err != nil {
		return nil, err
	}
	for id, path := range models.SharedVoices {
		voices[id] = &voiceSet{model: voices[kokoroEnglish].model, modelName: voices[kokoroEnglish].modelName, voiceName: stem(path)}
	}
	if models.ZhConfig != "" {
		if err := load(kokoroChinese, models.ZhWeights, models.ZhConfig, models.ZhVoice); err != nil {
			return nil, err
		}
	}
	ids := models.ExternalEncoders
	if len(ids) == 0 {
		ids = []string{encoder.IDWavLMBasePlusL6}
	}
	store := encoder.NewExternalStore()
	reg := encoder.NewRegistry()
	for _, id := range ids {
		if !encoderEnabled(id) {
			continue
		}
		switch id {
		case encoder.IDWavLMBasePlusL6:
			reg.Register(store.Encoder(id, 768, encoder.DefaultFrameMS, encoder.ConvFrameCount))
		case encoder.IDWav2Vec2EspeakL20:
			reg.Register(store.Encoder(id, 1024, encoder.DefaultFrameMS, encoder.ConvFrameCount))
		case encoder.IDWhisperSmallL12PosInv:
			reg.Register(store.Encoder(id, 768, encoder.DefaultFrameMS, encoder.WhisperFrameCount))
		default:
			return nil, fmt.Errorf("unknown external encoder %q", id)
		}
	}
	if len(reg.IDs()) == 0 {
		return nil, errors.New("no external reference encoders enabled")
	}
	return &Engine{voices: voices, registry: reg, external: true, externalStore: store,
		pending: map[string]Reference{}, prefetched: map[string]bool{}}, nil
}

func stem(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// readJobは、まだ結果のない読みを外に頼む。
func (e *Engine) readJob(set *voiceSet, key, phonemes string, speed float32, durations []int, withAudio bool, skeleton Reference) error {
	ids, style, err := set.model.Inputs(phonemes, durations)
	if err != nil {
		return err
	}
	e.readsMu.Lock()
	e.pending[key] = skeleton
	e.readsMu.Unlock()
	return &NeedInference{Jobs: []InferenceJob{{Kind: "kokoro", Key: key, Model: set.modelName, Voice: set.voiceName,
		Style: style, Tokens: ids, Speed: speed, Durations: durations, Audio: withAudio}}}
}

// ProvideReadは、外で求めた読みを渡す。audioがnilなら長さとF0だけ。
func (e *Engine) ProvideRead(key string, durations []int, f0, audio []float32) error {
	e.readsMu.Lock()
	defer e.readsMu.Unlock()
	for _, entry := range e.reads {
		if entry.key == key {
			entry.reference.Durations = durations
			entry.reference.F0 = f0
			if audio != nil {
				entry.reference.Audio = audio
			}
			// 波形だけを後から頼んだときの待ち合わせは、この結果で満たされた。
			delete(e.pending, key)
			return nil
		}
	}
	reference, ok := e.pending[key]
	if !ok {
		return fmt.Errorf("unknown reference read %q", key)
	}
	delete(e.pending, key)
	reference.Durations, reference.F0, reference.Audio = durations, f0, audio
	e.reads = append(e.reads, &cachedRead{key: key, reference: reference})
	if len(e.reads) > cachedReads {
		e.reads = e.reads[1:]
	}
	return nil
}

// ProvideFeatures supplies one external encoder result.
func (e *Engine) ProvideFeatures(encoderID, key string, values []float32, frames, dims int) error {
	if dims <= 0 || frames < 0 || frames > math.MaxInt/dims || frames*dims != len(values) {
		return fmt.Errorf("features %q: %d values for %d×%d", key, len(values), frames, dims)
	}
	if e.externalStore == nil {
		return errors.New("external feature inference is disabled")
	}
	if encoderID == "" {
		encoderID = encoder.IDWavLMBasePlusL6
	}
	return e.externalStore.Provide(encoderID, key, values, frames, dims)
}

func wrapNeedInference(err error) error {
	var need *encoder.NeedInference
	if !errors.As(err, &need) {
		return err
	}
	jobs := make([]InferenceJob, len(need.Jobs))
	for i, job := range need.Jobs {
		jobs[i] = InferenceJob{Kind: "features", EncoderID: job.EncoderID, Key: job.Key, Samples: job.Samples}
	}
	return &NeedInference{Jobs: jobs}
}

// ProvidePrefetchedは、filesで頼んだ録音を取得し終えたことを渡す。
func (e *Engine) ProvidePrefetched(key string) {
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	e.prefetched[key] = true
}

// filesJobは、索引を作る前に録音をまとめて取得するよう頼む。Web版の音源は読むときに1つずつ取得するため、
// 索引を作る途中で読むと長く止まる。取得済みならnilを返す。
func (e *Engine) filesJob(key string, files []speechpool.Entry) error {
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	if e.prefetched[key] {
		return nil
	}
	paths := make([]string, len(files))
	for i, file := range files {
		paths[i] = file.Path
	}
	return &NeedInference{Jobs: []InferenceJob{{Kind: "files", Key: key, Paths: paths}}}
}
