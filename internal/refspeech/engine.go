package refspeech

import (
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yh2237/UtauTTS/internal/hubert"
	"github.com/yh2237/UtauTTS/internal/kokoro"
	"github.com/yh2237/UtauTTS/internal/refmatch"
	"github.com/yh2237/gograd/dsp"
	"github.com/yh2237/gograd/torchrng"
)

const (
	hubertRate = 16000
	kokoroRate = 24000
	// DurationUnitMSはKokoroの音素長の単位（600サンプル）。
	DurationUnitMS = 25.0
	// kokoroSamplesPerTokenは長さ1あたりのKokoroの出力のサンプル数。
	kokoroSamplesPerToken = 600
	kokoroF0FrameMS       = 12.5
	kokoroSeed            = 0
	// F0FrameMSはSpeech.F0の周期。
	F0FrameMS = refmatch.FrameMS / 4
)

// FeatureModelは索引の特徴量の種類。変えたら索引を作り直す。
// UTAUTTS_FEATURE_MODELとUTAUTTS_HUBERT_LAYERで変えられる（実験用）。
var (
	FeatureModel = "wavlm-base-plus-layer6"
	hubertLayer  = 6
)

func init() {
	layerChanged := false
	if value := os.Getenv("UTAUTTS_HUBERT_LAYER"); value != "" {
		layer, err := strconv.Atoi(value)
		if err == nil && layer > 0 {
			hubertLayer = layer
			layerChanged = true
		}
	}
	if name := os.Getenv("UTAUTTS_FEATURE_MODEL"); name != "" {
		FeatureModel = fmt.Sprintf("%s-layer%d", name, hubertLayer)
	} else if layerChanged {
		FeatureModel = fmt.Sprintf("wavlm-base-plus-layer%d", hubertLayer)
	}
}

// Modelsは同梱するモデルのファイル。
type Models struct {
	KokoroWeights string
	KokoroConfig  string
	KokoroVoice   string
	// ZhWeights、ZhConfig、ZhVoiceは中国語のKokoro。空なら中国語の参照は使わない。
	ZhWeights string
	ZhConfig  string
	ZhVoice   string
	// SharedVoicesは、英語と同じKokoro v1.0のモデルで読む声（参照TTSのIDごとの声のファイル）。
	SharedVoices map[string]string
	HuBERT       string
}

// 参照TTSのID。Provider.ID()と対応する。
const (
	kokoroEnglish    = "kokoro-en"
	kokoroChinese    = "kokoro-zh"
	kokoroSpanish    = "kokoro-es"
	kokoroPortuguese = "kokoro-pt"
)

// voiceSetは1つの参照TTSのモデルと声。
type voiceSet struct {
	model *kokoro.Model
	voice *kokoro.Voice
}

type Engine struct {
	// voicesは参照TTSのIDごとのモデルと声。
	voices map[string]*voiceSet
	hubert *hubert.Model

	// readsは最近のKokoroの読み。予測と合成で同じ文を読み直さないために使う。
	readsMu sync.Mutex
	reads   []*cachedRead

	// featuresは最近の参照音声の特徴量。同じ文を続けて合成するときに計算し直さない。
	featuresMu sync.Mutex
	features   []*cachedFeatures
}

type cachedRead struct {
	key       string
	reference Reference
	prosody   *kokoro.Prosody
}

type cachedFeatures struct {
	key    string
	values []float32
	frames int
	dims   int
}

const cachedReads = 8

func Open(models Models) (*Engine, error) {
	if path := os.Getenv("UTAUTTS_HUBERT_PATH"); path != "" {
		models.HuBERT = path
	}
	voices := map[string]*voiceSet{}
	english, err := loadVoiceSet(kokoroEnglish, models.KokoroWeights, models.KokoroConfig, models.KokoroVoice)
	if err != nil {
		return nil, err
	}
	voices[kokoroEnglish] = english
	for id, path := range models.SharedVoices {
		voice, err := kokoro.LoadVoice(path)
		if err != nil {
			return nil, fmt.Errorf("load Kokoro voice (%s): %w", id, err)
		}
		voices[id] = &voiceSet{model: english.model, voice: voice}
	}
	if models.ZhWeights != "" && models.ZhConfig != "" && models.ZhVoice != "" {
		chinese, err := loadVoiceSet(kokoroChinese, models.ZhWeights, models.ZhConfig, models.ZhVoice)
		if err != nil {
			return nil, err
		}
		voices[kokoroChinese] = chinese
	}
	features, err := hubert.Load(models.HuBERT)
	if err != nil {
		return nil, fmt.Errorf("load HuBERT: %w", err)
	}
	return &Engine{voices: voices, hubert: features}, nil
}

// loadVoiceSetは、1つのKokoroのモデルと声を読む。
func loadVoiceSet(id, weights, config, voicePath string) (*voiceSet, error) {
	model, err := kokoro.Load(weights, config)
	if err != nil {
		return nil, fmt.Errorf("load Kokoro (%s): %w", id, err)
	}
	voice, err := kokoro.LoadVoice(voicePath)
	if err != nil {
		return nil, fmt.Errorf("load Kokoro voice (%s): %w", id, err)
	}
	return &voiceSet{model: model, voice: voice}, nil
}

// voiceSetForは、参照TTSのIDに対応するモデルと声を返す。
func (e *Engine) voiceSetFor(id string) (*voiceSet, error) {
	if id == "" {
		id = kokoroEnglish
	}
	set := e.voices[id]
	if set == nil {
		return nil, fmt.Errorf("reference TTS %q is not installed", id)
	}
	return set, nil
}

// FeaturesはHuBERT Baseの6層目を20msごとに返す。refmatch.FeatureFuncとして使う。
func (e *Engine) Features(samples []float64, sampleRate int) ([]float32, int, int, error) {
	wave := make([]float32, len(samples))
	for i, v := range samples {
		wave[i] = float32(v)
	}
	rows, err := e.hubert.FeaturesAccelerated(dsp.Resample(wave, sampleRate, hubertRate), hubertLayer)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(rows) == 0 {
		return nil, 0, 0, errors.New("audio is too short for HuBERT")
	}
	dims := len(rows[0])
	values := make([]float32, 0, len(rows)*dims)
	for _, row := range rows {
		values = append(values, row...)
	}
	return values, len(rows), dims, nil
}

type Options struct {
	// SpeedはKokoroの話速。1が標準。Durationsを指定したときは使わない。
	Speed float32
	// Durationsは入力記号ごとの長さ（Timing.Durationsと同じ並び）。nilならKokoroが予測する。
	Durations []int
	// CenterHzは出力のF0の中心。0なら音源のF0の中央値。
	CenterHz float64
	// PitchScaleはKokoroのF0の動きを何倍にするか。
	PitchScale float64
	// PitchCentsは、最初の記号の始まりからのミリ秒ごとに、出力のF0へ足すセント。nilなら足さない。
	PitchCents func(ms float64) float64
	// ReplacePitchなら、PitchCentsをF0の中心からのセントとして、KokoroのF0の動きの代わりに使う。
	ReplacePitch bool
	// Logfは合成の途中経過を伝える。nilなら何も伝えない。
	Logf func(string)
}

// TimingはKokoroの入力記号（先頭と末尾の境界記号を含む）ごとの長さ。
type Timing struct {
	// Durationsは記号ごとの長さ（600サンプル単位）。
	Durations []int
	// PieceTokensは音素列の断片ごとの記号の範囲（Durationsの添字、終わりを含まない）。
	PieceTokens [][2]int
}

// MSは記号の長さをミリ秒で返す。
func (t Timing) MS(token int) float64 { return float64(t.Durations[token]) * DurationUnitMS }

// LeadingMSは、音声の中で最初のモーラが始まる時刻。
// Kokoroの音声は記号の長さを並べた時刻より約50ms早く鳴るため、先頭の境界記号の長さから差し引く
// （日本語・英語、話速によらない。強制整列で測った）。
func (t Timing) LeadingMS() float64 {
	if len(t.Durations) == 0 {
		return 0
	}
	return max(0, t.MS(0)-kokoroAudioLeadMS)
}

const kokoroAudioLeadMS = 50.0

type Speech struct {
	Audio      []float64
	SampleRate int
	Timing
	// F0は出力のF0（Hz、F0FrameMSごと、0は無声）。AutomaticF0は手動の抑揚を足す前のF0。
	F0, AutomaticF0 []float64
}

// ReferenceはKokoroの読み。
type Reference struct {
	// Audioは24kHzの音声（ReadProsodyではnil）。F0はKokoroの内部のF0（12.5msごと、Hz）。
	Audio []float32
	F0    []float32
	Timing
	// SampleRateはAudioの標本化周波数。0なら24kHz。
	SampleRate int
	// Keyは同じ読みを見分けるための値。特徴量の使い回しに使う（中身は問わない）。
	Key string
}

// ReferenceF0は参照のF0を、有声区間を約20msでなめらかにしてF0FrameMSごとにする（0は無声）。
func (r Reference) ReferenceF0() []float64 {
	samples := 0
	for _, d := range r.Durations {
		samples += d * kokoroSamplesPerToken
	}
	frames := samples * 1000 / kokoroRate / int(refmatch.FrameMS)
	return refmatch.ReferenceF0(r.F0, kokoroF0FrameMS, frames)
}

// Readは音素列を参照TTSで読む。durationsを渡すと、その長さで読む。返す値は書き換えない。
func (e *Engine) Read(id string, pieces []Piece, speed float32, durations []int) (Reference, error) {
	return e.read(id, pieces, speed, durations, true)
}

// readKeyは、Kokoroの読みと、その音声から作る特徴量を使い回すためのキー。
func readKey(id string, symbols []string, speed float32, durations []int) string {
	return fmt.Sprint(id, "\x02", strings.Join(symbols, "\x00"), "\x01", speed, "\x01", durations)
}

// cachedFeaturesは、同じ読みの参照音声から作った特徴量を返す。
func (e *Engine) cachedFeatures(key string) *cachedFeatures {
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	for _, entry := range e.features {
		if entry.key == key {
			return entry
		}
	}
	return nil
}

// storeFeaturesは、参照音声の特徴量を覚えておく。
func (e *Engine) storeFeatures(key string, values []float32, frames, dims int) {
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	entry := &cachedFeatures{key: key, values: values, frames: frames, dims: dims}
	for i, cached := range e.features {
		if cached.key == key {
			e.features[i] = entry
			return
		}
	}
	e.features = append(e.features, entry)
	if len(e.features) > cachedReads {
		e.features = e.features[1:]
	}
}

// ReadProsodyはReadと同じく読むが、波形は作らずに記号ごとの長さとF0だけを返す。
// 後で同じ引数でReadを呼ぶと、ここで求めた結果から波形だけを作る。
func (e *Engine) ReadProsody(id string, pieces []Piece, speed float32, durations []int) (Reference, error) {
	return e.read(id, pieces, speed, durations, false)
}

func (e *Engine) read(id string, pieces []Piece, speed float32, durations []int, withAudio bool) (Reference, error) {
	set, err := e.voiceSetFor(id)
	if err != nil {
		return Reference{}, err
	}
	var phonemes strings.Builder
	symbols := make([]string, len(pieces))
	for i, piece := range pieces {
		phonemes.WriteString(piece.Symbols)
		symbols[i] = piece.Symbols
	}
	if speed <= 0 {
		speed = 1
	}
	key := readKey(id, symbols, speed, durations)
	e.readsMu.Lock()
	var entry *cachedRead
	for _, cached := range e.reads {
		if cached.key == key {
			entry = cached
		}
	}
	e.readsMu.Unlock()
	if entry == nil {
		prosody, err := set.model.ReadProsody(phonemes.String(), set.voice, speed, durations)
		if err != nil {
			return Reference{}, err
		}
		entry = &cachedRead{key: key, prosody: prosody, reference: Reference{F0: prosody.F0, SampleRate: kokoroRate, Key: key,
			Timing: Timing{Durations: prosody.Durations, PieceTokens: make([][2]int, len(pieces))}}}
		token := 1
		for i, piece := range pieces {
			start := token
			token += len(set.model.Tokens(piece.Symbols))
			entry.reference.PieceTokens[i] = [2]int{start, token}
		}
		e.readsMu.Lock()
		e.reads = append(e.reads, entry)
		if len(e.reads) > cachedReads {
			e.reads = e.reads[1:]
		}
		e.readsMu.Unlock()
	}
	e.readsMu.Lock()
	reference := entry.reference
	e.readsMu.Unlock()
	if !withAudio || reference.Audio != nil {
		return reference, nil
	}
	audio, err := set.model.Decode(entry.prosody, torchrng.New(kokoroSeed))
	if err != nil {
		return Reference{}, err
	}
	e.readsMu.Lock()
	entry.reference.Audio = audio
	reference = entry.reference
	e.readsMu.Unlock()
	return reference, nil
}

// Speakは音素列を参照TTSで読み、その特徴量に近い音源のフレームで合成し直す。
func (e *Engine) Speak(id string, index *refmatch.Index, world refmatch.World, pieces []Piece, options Options) (Speech, error) {
	reference, err := e.Read(id, pieces, options.Speed, options.Durations)
	if err != nil {
		return Speech{}, err
	}
	return e.SpeakReference(index, world, reference, options)
}

// SpeakReferenceは、参照の読み（Reference）から、音源のフレームで合成し直す。
// 参照TTSが何であっても、音声とF0があれば同じように合成できる。
func (e *Engine) SpeakReference(index *refmatch.Index, world refmatch.World, reference Reference, options Options) (Speech, error) {
	if index.Model != FeatureModel {
		return Speech{}, fmt.Errorf("voicebank index uses %q, want %q", index.Model, FeatureModel)
	}
	rate := reference.SampleRate
	if rate <= 0 {
		rate = kokoroRate
	}
	samples := make([]float64, len(reference.Audio))
	for i, v := range reference.Audio {
		samples[i] = float64(v)
	}
	var features []float32
	var frames int
	featuresFromCache := false
	if reference.Key != "" {
		if cached := e.cachedFeatures(reference.Key); cached != nil {
			features, frames = cached.values, cached.frames
			featuresFromCache = true
		}
	}
	featuresStarted := time.Now()
	if features == nil {
		SetStage("特徴量を計算しています")
		var dims int
		var err error
		if features, frames, dims, err = e.Features(samples, rate); err != nil {
			return Speech{}, err
		}
		if reference.Key != "" {
			e.storeFeatures(reference.Key, features, frames, dims)
		}
	}
	featuresElapsed := time.Since(featuresStarted)
	referenceF0 := refmatch.ReferenceF0(reference.F0, kokoroF0FrameMS, frames)
	levels := refmatch.ReferenceLevels(samples, rate, frames*4)
	// 参照TTSのF0は語尾で途切れることがある。音量がある区間は有声として扱う。
	refmatch.FillVoicedByLevel(referenceF0, levels, -35)
	automaticF0 := index.OutputF0(referenceF0, options.CenterHz, options.PitchScale)
	outF0 := append([]float64(nil), automaticF0...)
	if options.PitchCents != nil {
		center := options.CenterHz
		if center <= 0 {
			center = index.F0Median
		}
		leading := reference.LeadingMS()
		for i, v := range outF0 {
			if v <= 0 {
				continue
			}
			cents := options.PitchCents(float64(i)*F0FrameMS - leading)
			if options.ReplacePitch {
				outF0[i] = center * math.Exp2(cents/1200)
			} else {
				outF0[i] = v * math.Exp2(cents/1200)
			}
		}
	}
	synthesizeStarted := time.Now()
	SetStage("照合と波形の合成をしています")
	audio, err := index.Synthesize(world, refmatch.Request{
		Features: features, Frames: frames, ReferenceF0: referenceF0, OutputF0: outF0,
		ReferenceLevel: levels,
	})
	if err != nil {
		return Speech{}, err
	}
	if options.Logf != nil {
		source := "計算"
		if featuresFromCache {
			source = "キャッシュ"
		}
		options.Logf(fmt.Sprintf("合成: 特徴量 %s %d ms、照合と波形 %d ms（%d フレーム、%d サンプル）",
			source, featuresElapsed.Milliseconds(), time.Since(synthesizeStarted).Milliseconds(),
			frames, len(audio)))
	}
	return Speech{Audio: audio, SampleRate: index.SampleRate, Timing: reference.Timing, F0: outF0, AutomaticF0: automaticF0}, nil
}

// SpeakAudioは参照の音声から、音源のフレームで合成し直す。言語に依存しない入口。
// 音素の並びや長さは使わず、参照の音声と、そこから測ったF0だけを使う。
func (e *Engine) SpeakAudio(index *refmatch.Index, world refmatch.World, audio []float64, sampleRate int) (Speech, error) {
	if index.Model != FeatureModel {
		return Speech{}, fmt.Errorf("voicebank index uses %q, want %q", index.Model, FeatureModel)
	}
	f0, _, _, _, err := world.Analyze(audio, sampleRate, refmatch.FrameMS, 0.85)
	if err != nil {
		return Speech{}, fmt.Errorf("analyze reference: %w", err)
	}
	features, frames, _, err := e.Features(audio, sampleRate)
	if err != nil {
		return Speech{}, err
	}
	f0Values := make([]float32, len(f0))
	for i, v := range f0 {
		f0Values[i] = float32(v)
	}
	referenceF0 := refmatch.ReferenceF0(f0Values, refmatch.FrameMS, frames)
	automaticF0 := index.OutputF0(referenceF0, 0, 1)
	rendered, err := index.Synthesize(world, refmatch.Request{
		Features: features, Frames: frames, ReferenceF0: referenceF0, OutputF0: automaticF0,
		ReferenceLevel: refmatch.ReferenceLevels(audio, sampleRate, frames*4),
	})
	if err != nil {
		return Speech{}, err
	}
	return Speech{Audio: rendered, SampleRate: index.SampleRate, F0: automaticF0}, nil
}
