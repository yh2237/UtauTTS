package refspeech

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/yh2237/UtauIDX/encoder"
	"github.com/yh2237/UtauIDX/index"
	"github.com/yh2237/UtauIDX/matcher"
	"github.com/yh2237/UtauIDX/vocoder"
	"github.com/yh2237/UtauTTS/internal/kokoro"
	"github.com/yh2237/gograd/torchrng"
)

const (
	F0FrameMS             = index.WorldFrameMS
	DurationUnitMS        = 25.0
	kokoroRate            = 24000
	kokoroF0FrameMS       = 12.5
	kokoroSamplesPerToken = 600
	kokoroSeed            = 0
	kokoroAudioLeadMS     = 50.0
)

type Models struct {
	KokoroWeights      string
	KokoroConfig       string
	KokoroVoice        string
	ZhWeights          string
	ZhConfig           string
	ZhVoice            string
	SharedVoices       map[string]string
	WavLM              string
	Espeak             string
	EspeakConfig       string
	EspeakPreprocessor string
	Whisper            string
	WhisperConfig      string
	ExternalEncoders   []string
}

const (
	kokoroEnglish    = "kokoro-en"
	kokoroChinese    = "kokoro-zh"
	kokoroSpanish    = "kokoro-es"
	kokoroPortuguese = "kokoro-pt"
)

type voiceSet struct {
	model *kokoro.Model
	voice *kokoro.Voice
	// modelNameとvoiceNameは、推論を外に頼むときに渡すファイル名。
	modelName, voiceName string
}

type Engine struct {
	voices        map[string]*voiceSet
	registry      *encoder.Registry
	external      bool
	externalStore *encoder.ExternalStore

	readsMu sync.Mutex
	reads   []*cachedRead
	pending map[string]Reference

	featuresMu sync.Mutex
	features   []*cachedQueryFeatures
	prefetched map[string]bool
}

type cachedRead struct {
	key       string
	reference Reference
	prosody   *kokoro.Prosody
}

type cachedQueryFeatures struct {
	key      string
	channels map[string][]float32
	frames   int
	dims     map[string]int
}

const cachedReads = 8

func Open(models Models) (engine *Engine, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			engine, err = nil, fmt.Errorf("参照照合のモデルを読み込めませんでした: %v", recovered)
		}
	}()
	if externalInference {
		return openExternal(models)
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
	reg, err := loadEncoders(models)
	if err != nil {
		return nil, err
	}
	return &Engine{voices: voices, registry: reg}, nil
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

type Options struct {
	// ContinuityBonusは同じ録音の続きのフレームを優先する強さ。nilなら既定値を使う。
	ContinuityBonus *float64
	// SpeedはKokoroの話速。1が標準。Durationsを指定したときは使わない。
	Speed float32
	// Durationsは入力記号ごとの長さ（Timing.Durationsと同じ並び）。nilならKokoroが予測する。
	Durations []int
	// Warpは参照の時間軸を出力の時間軸へ引き伸ばす対応。nilなら参照の時間軸のまま。
	// Durationsと違い、読み直さずに長さを変えられる。
	Warp *Warp
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
	frames := samples * 1000 / kokoroRate / int(index.FrameMS)
	return vocoder.ReferenceF0(r.F0, kokoroF0FrameMS, frames)
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
func (e *Engine) cachedFeatures(key string) *cachedQueryFeatures {
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
func (e *Engine) storeFeatures(key string, channels map[string][]float32, frames int, dims map[string]int) {
	e.featuresMu.Lock()
	defer e.featuresMu.Unlock()
	entry := &cachedQueryFeatures{key: key, channels: channels, frames: frames, dims: dims}
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
	if entry == nil && e.external {
		skeleton := Reference{SampleRate: kokoroRate, Key: key, Timing: Timing{PieceTokens: pieceTokens(set.model, pieces)}}
		return Reference{}, e.readJob(set, key, phonemes.String(), speed, durations, withAudio, skeleton)
	}
	if entry == nil {
		prosody, err := set.model.ReadProsody(phonemes.String(), set.voice, speed, durations)
		if err != nil {
			return Reference{}, err
		}
		entry = &cachedRead{key: key, prosody: prosody, reference: Reference{F0: prosody.F0, SampleRate: kokoroRate, Key: key,
			Timing: Timing{Durations: prosody.Durations, PieceTokens: pieceTokens(set.model, pieces)}}}
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
	if e.external {
		return Reference{}, e.readJob(set, key, phonemes.String(), speed, reference.Durations, true, reference)
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
func (e *Engine) Speak(id string, idx *index.Index, world index.World, pieces []Piece, options Options) (Speech, error) {
	reference, err := e.Read(id, pieces, options.Speed, options.Durations)
	if err != nil {
		return Speech{}, err
	}
	return e.SpeakReference(idx, world, reference, options)
}

func (e *Engine) SpeakReference(idx *index.Index, world index.World, reference Reference, options Options) (Speech, error) {
	if err := validateSynthesis(idx, world, options.ContinuityBonus); err != nil {
		return Speech{}, err
	}
	rate := reference.SampleRate
	if rate <= 0 {
		rate = kokoroRate
	}
	samples := make([]float64, len(reference.Audio))
	for i, v := range reference.Audio {
		samples[i] = float64(v)
	}
	started := time.Now()
	channels, dims, frames, err := e.queryFeatures(idx, samples, rate, reference.Key)
	if err != nil {
		return Speech{}, err
	}
	featuresElapsed := time.Since(started)
	analysis, err := vocoder.AnalyzeReference(world, samples, rate, frames, reference.F0, kokoroF0FrameMS)
	if err != nil {
		return Speech{}, err
	}
	warpQuery(options.Warp, reference.LeadingMS(), channels, dims, &analysis)
	vocoder.FillVoicedByLevel(analysis.F0, analysis.Levels, -35)
	automaticF0 := outputF0(idx, analysis.F0, options.CenterHz, options.PitchScale)
	outF0 := append([]float64(nil), automaticF0...)
	if options.PitchCents != nil {
		center := options.CenterHz
		if center <= 0 {
			center = idx.F0Median
		}
		leading := reference.LeadingMS()
		for i, v := range outF0 {
			if v <= 0 {
				continue
			}
			cents := options.PitchCents(float64(i)*F0FrameMS - leading)
			if options.ReplacePitch {
				v = center
			}
			outF0[i] = v * math.Exp2(cents/1200)
		}
	}
	audio, err := synthesizeQuery(idx, world, channels, analysis, outF0, options.ContinuityBonus, options.Logf)
	if err != nil {
		return Speech{}, err
	}
	if options.Logf != nil {
		options.Logf(fmt.Sprintf("合成: 特徴量 %d ms、全体 %d ms（%d フレーム、%d サンプル）",
			featuresElapsed.Milliseconds(), time.Since(started).Milliseconds(), analysis.Frames, len(audio)))
	}
	return Speech{Audio: audio, SampleRate: idx.SampleRate, Timing: reference.Timing, F0: outF0, AutomaticF0: automaticF0}, nil
}

func validateSynthesis(idx *index.Index, world index.World, continuity *float64) error {
	if idx == nil || world == nil {
		return errors.New("reference synthesis needs an index and WORLD engine")
	}
	if continuity != nil && (*continuity < 0 || math.IsNaN(*continuity) || math.IsInf(*continuity, 0)) {
		return errors.New("continuity bonus must be finite and nonnegative")
	}
	return nil
}

func outputF0(idx *index.Index, f0 []float64, centerHz, pitchScale float64) []float64 {
	if pitchScale == 0 {
		pitchScale = 1
	}
	return vocoder.OutputF0(f0, idx.F0Median, centerHz, pitchScale)
}

func synthesizeQuery(idx *index.Index, world index.World, channels map[string][]float32, analysis vocoder.ReferenceAnalysis,
	outF0 []float64, continuity *float64, logf func(string)) ([]float64, error) {
	SetStage("照合と波形の合成をしています")
	query := matcher.Query{
		Frames: analysis.Frames, Channels: channels, MouthStatic: analysis.MouthStatic,
		MouthDelta: analysis.MouthDelta, Level5ms: analysis.Levels,
	}
	profile := matcher.ComputeAutoProfile(idx, query)
	cfg := vocoder.AutoSynthConfig(profile)
	query.Voiced = analysis.Voiced20ms(cfg.UseSafeRefAP)
	result, err := matcher.Match(idx, query, matcher.Options{Profile: &profile, ContinuityBonus: continuity})
	if err != nil {
		return nil, err
	}
	audio, err := (&vocoder.WorldVocoder{World: world}).Synthesize(idx, result, vocoder.SynthRequest{
		ReferenceF0: analysis.F0, OutputF0: outF0, ReferenceLevel: analysis.Levels,
		ReferenceAP: analysis.SafeRefAP, Config: &cfg,
	})
	if err != nil {
		return nil, err
	}
	if logf != nil {
		logf(fmt.Sprintf("Profile: Gap=%.6f Xling=%.6f RawPool=%t Weights=%v SafeRefAP=%t", profile.Gap, profile.Xling, profile.IsRawPool, profile.ChannelWeights, cfg.UseSafeRefAP))
		stats := result.Stats
		logf(fmt.Sprintf("SelectStats (top1, 5ms): Frames=%d Runs=%d Switches=%d MeanRunFrames=%.6f UniqueUnits=%d",
			stats.Frames, stats.Runs, stats.Switches, stats.MeanRunFrames, stats.UniqueUnits))
	}
	return audio, nil
}

// pieceTokensは、区切りごとの記号の範囲（先頭の0を数えた位置）を返す。
func pieceTokens(model *kokoro.Model, pieces []Piece) [][2]int {
	spans := make([][2]int, len(pieces))
	token := 1
	for i, piece := range pieces {
		start := token
		token += len(model.Tokens(piece.Symbols))
		spans[i] = [2]int{start, token}
	}
	return spans
}
