package refspeech

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/refmatch"
	"github.com/yh2237/gograd/dsp"
	"github.com/yh2237/gograd/hubert"
	"github.com/yh2237/gograd/kokoro"
	"github.com/yh2237/gograd/torchrng"
)

const (
	// FeatureModelは索引の特徴量の種類。変えたら索引を作り直す。
	FeatureModel = "hubert-base-ls960-layer6"
	hubertLayer  = 6
	hubertRate   = 16000
	kokoroRate   = 24000
	// DurationUnitMSはKokoroの音素長の単位（600サンプル）。
	DurationUnitMS  = 25.0
	kokoroF0FrameMS = 12.5
	kokoroSeed      = 0
	// F0FrameMSはSpeech.F0の周期。
	F0FrameMS = refmatch.FrameMS / 4
)

// Modelsは同梱するモデルのファイル。
type Models struct {
	KokoroWeights string
	KokoroConfig  string
	KokoroVoice   string
	HuBERT        string
}

type Engine struct {
	kokoro *kokoro.Model
	voice  *kokoro.Voice
	hubert *hubert.Model

	// readsは最近のKokoroの読み。予測と合成で同じ文を読み直さないために使う。
	readsMu sync.Mutex
	reads   []cachedRead
}

type cachedRead struct {
	key       string
	reference Reference
}

const cachedReads = 8

func Open(models Models) (*Engine, error) {
	model, err := kokoro.Load(models.KokoroWeights, models.KokoroConfig)
	if err != nil {
		return nil, fmt.Errorf("load Kokoro: %w", err)
	}
	voice, err := kokoro.LoadVoice(models.KokoroVoice)
	if err != nil {
		return nil, fmt.Errorf("load Kokoro voice: %w", err)
	}
	features, err := hubert.Load(models.HuBERT)
	if err != nil {
		return nil, fmt.Errorf("load HuBERT: %w", err)
	}
	return &Engine{kokoro: model, voice: voice, hubert: features}, nil
}

func (e *Engine) Close() error { return e.hubert.Close() }

// FeaturesはHuBERT Baseの6層目を20msごとに返す。refmatch.FeatureFuncとして使う。
func (e *Engine) Features(samples []float64, sampleRate int) ([]float32, int, int, error) {
	wave := make([]float32, len(samples))
	for i, v := range samples {
		wave[i] = float32(v)
	}
	rows, err := e.hubert.Features(dsp.Resample(wave, sampleRate, hubertRate), hubertLayer)
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

// LeadingMSは先頭の境界記号の長さ。最初のモーラはここから始まる。
func (t Timing) LeadingMS() float64 {
	if len(t.Durations) == 0 {
		return 0
	}
	return t.MS(0)
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
	// Audioは24kHzの音声。F0はKokoroの内部のF0（12.5msごと、Hz）。
	Audio []float32
	F0    []float32
	Timing
}

// ReferenceF0は参照のF0を、有声区間を約20msでなめらかにしてF0FrameMSごとにする（0は無声）。
func (r Reference) ReferenceF0() []float64 {
	frames := len(r.Audio) * 1000 / kokoroRate / int(refmatch.FrameMS)
	return refmatch.ReferenceF0(r.F0, kokoroF0FrameMS, frames)
}

// Readは音素列をKokoroで読む。durationsを渡すと、その長さで読む。返す値は書き換えない。
func (e *Engine) Read(pieces []Piece, speed float32, durations []int) (Reference, error) {
	var phonemes strings.Builder
	symbols := make([]string, len(pieces))
	for i, piece := range pieces {
		phonemes.WriteString(piece.Symbols)
		symbols[i] = piece.Symbols
	}
	if speed <= 0 {
		speed = 1
	}
	key := fmt.Sprint(strings.Join(symbols, "\x00"), "\x01", speed, "\x01", durations)
	e.readsMu.Lock()
	for _, cached := range e.reads {
		if cached.key == key {
			e.readsMu.Unlock()
			return cached.reference, nil
		}
	}
	e.readsMu.Unlock()
	var result kokoro.Result
	var err error
	if durations != nil {
		result, err = e.kokoro.SynthesizeWithDurations(phonemes.String(), e.voice, durations, torchrng.New(kokoroSeed))
	} else {
		result, err = e.kokoro.Synthesize(phonemes.String(), e.voice, speed, torchrng.New(kokoroSeed))
	}
	if err != nil {
		return Reference{}, err
	}
	reference := Reference{Audio: result.Audio, F0: result.F0,
		Timing: Timing{Durations: result.Durations, PieceTokens: make([][2]int, len(pieces))}}
	token := 1
	for i, piece := range pieces {
		start := token
		token += len(e.kokoro.Tokens(piece.Symbols))
		reference.PieceTokens[i] = [2]int{start, token}
	}
	e.readsMu.Lock()
	e.reads = append(e.reads, cachedRead{key: key, reference: reference})
	if len(e.reads) > cachedReads {
		e.reads = e.reads[1:]
	}
	e.readsMu.Unlock()
	return reference, nil
}

// Speakは音素列をKokoroで読み、その特徴量に近い音源のフレームで合成し直す。
func (e *Engine) Speak(index *refmatch.Index, world refmatch.World, pieces []Piece, options Options) (Speech, error) {
	if index.Model != FeatureModel {
		return Speech{}, fmt.Errorf("voicebank index uses %q, want %q", index.Model, FeatureModel)
	}
	reference, err := e.Read(pieces, options.Speed, options.Durations)
	if err != nil {
		return Speech{}, err
	}
	samples := make([]float64, len(reference.Audio))
	for i, v := range reference.Audio {
		samples[i] = float64(v)
	}
	features, frames, _, err := e.Features(samples, kokoroRate)
	if err != nil {
		return Speech{}, err
	}
	referenceF0 := refmatch.ReferenceF0(reference.F0, kokoroF0FrameMS, frames)
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
	audio, err := index.Synthesize(world, refmatch.Request{
		Features: features, Frames: frames, ReferenceF0: referenceF0, OutputF0: outF0,
	})
	if err != nil {
		return Speech{}, err
	}
	return Speech{Audio: audio, SampleRate: index.SampleRate, Timing: reference.Timing, F0: outF0, AutomaticF0: automaticF0}, nil
}
