package refspeech

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"math"
	"path/filepath"
	"slices"
	"strings"

	"github.com/yh2237/UtauIDX/refmatch"
	"github.com/yh2237/UtauTTS/internal/kokoro"
	"github.com/yh2237/gograd/dsp"
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
	// Kindはkokoro（読み）、features（WavLMの6層目）、files（索引を作る前に録音をまとめて取得する）。
	Kind string `json:"kind"`
	Key  string `json:"key"`
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

type providedFeatures struct {
	values []float32
	frames int
	dims   int
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
	return &Engine{voices: voices, external: true, provided: map[string]providedFeatures{}, pending: map[string]Reference{},
		prefetched: map[string]bool{}}, nil
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

// ProvideFeaturesは、外で求めたWavLMの特徴量（frames×dims）を渡す。
func (e *Engine) ProvideFeatures(key string, values []float32, frames, dims int) error {
	// frames*dimsが桁あふれしないよう、割り算で先に確かめる。
	if dims <= 0 || frames < 0 || frames > len(values)/dims || frames*dims != len(values) {
		return fmt.Errorf("features %q: %d values for %d×%d", key, len(values), frames, dims)
	}
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	e.provided[key] = providedFeatures{values: values, frames: frames, dims: dims}
	return nil
}

// externalFeaturesは、16 kHzにした音声の中身をキーに、外で求めた特徴量を返す。
// 集める途中（collectingが非nil）は、頼む推論を記録して同じ形の0を返す。
func (e *Engine) externalFeatures(samples []float64, sampleRate int) ([]float32, int, int, error) {
	wave := make([]float32, len(samples))
	for i, v := range samples {
		wave[i] = float32(v)
	}
	wave = dsp.Resample(wave, sampleRate, hubertRate)
	key := featuresKey(wave)
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	if p, ok := e.provided[key]; ok {
		return p.values, p.frames, p.dims, nil
	}
	job := InferenceJob{Kind: "features", Key: key, Samples: wave}
	if e.collecting == nil {
		return nil, 0, 0, &NeedInference{Jobs: []InferenceJob{job}}
	}
	if !slices.ContainsFunc(*e.collecting, func(other InferenceJob) bool { return other.Key == key }) {
		*e.collecting = append(*e.collecting, job)
	}
	frames := wavlmFrames(len(wave))
	if frames < 1 {
		return nil, 0, 0, fmt.Errorf("audio is too short for HuBERT")
	}
	return make([]float32, frames*wavlmDims), frames, wavlmDims, nil
}

const wavlmDims = 768

// wavlmFramesはWavLMの畳み込み（窓400、間隔320）が出すフレーム数。
func wavlmFrames(samples int) int {
	if samples < 400 {
		return 0
	}
	return (samples-400)/320 + 1
}

func featuresKey(wave []float32) string {
	hash := fnv.New64a()
	var buffer [4]byte
	for _, v := range wave {
		binary.LittleEndian.PutUint32(buffer[:], math.Float32bits(v))
		hash.Write(buffer[:])
	}
	return fmt.Sprintf("f%d-%x", len(wave), hash.Sum64())
}

// collectFeaturesは、runの中で特徴量を求めようとした録音を集めて返す。runの結果は使わない。
func (e *Engine) collectFeatures(run func()) []InferenceJob {
	var jobs []InferenceJob
	e.featuresMu.Lock()
	e.collecting = &jobs
	e.featuresMu.Unlock()
	run()
	e.featuresMu.Lock()
	e.collecting = nil
	e.featuresMu.Unlock()
	return jobs
}

// ProvidePrefetchedは、filesで頼んだ録音を取得し終えたことを渡す。
func (e *Engine) ProvidePrefetched(key string) {
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	e.prefetched[key] = true
}

// filesJobは、索引を作る前に録音をまとめて取得するよう頼む。Web版の音源は読むときに1つずつ取得するため、
// 索引を作る途中で読むと長く止まる。取得済みならnilを返す。
func (e *Engine) filesJob(key string, files []refmatch.Recording) error {
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

// dropProvidedは、索引を作り終えた録音の特徴量を捨てる。
func (e *Engine) dropProvided() {
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	e.provided = map[string]providedFeatures{}
}

// dryWorldは、特徴量を頼む録音を集めるときに、WORLDの解析を省く。
type dryWorld struct{}

func (dryWorld) Analyze(samples []float64, sampleRate int, frameMS, _ float64) ([]float64, []float64, []float64, int, error) {
	const fftSize = 2048
	frames := int(float64(len(samples))/float64(sampleRate)*1000/frameMS) + 1
	bins := fftSize/2 + 1
	spectrum := make([]float64, frames*bins)
	aperiodicity := make([]float64, frames*bins)
	for i := range spectrum {
		spectrum[i], aperiodicity[i] = 1e-6, 1
	}
	return make([]float64, frames), spectrum, aperiodicity, fftSize, nil
}

func (dryWorld) Synthesize([]float64, []float64, []float64, int, int, float64) ([]float64, error) {
	return nil, fmt.Errorf("dry WORLD cannot synthesize")
}

var _ refmatch.World = dryWorld{}
