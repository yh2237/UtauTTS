package refspeech

import (
	"errors"
	"fmt"
	"strings"

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
	// kokoroDurationMSはKokoroの音素長の単位（600サンプル）。
	kokoroDurationMS = 25.0
	kokoroF0FrameMS  = 12.5
	kokoroSeed       = 0
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
}

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
	// SpeedはKokoroの話速。1が標準。
	Speed float32
	// CenterHzは出力のF0の中心。0なら音源のF0の中央値。
	CenterHz float64
	// PitchScaleはKokoroのF0の動きを何倍にするか。
	PitchScale float64
}

type Speech struct {
	Audio      []float64
	SampleRate int
	// PieceMSは音素列の断片ごとの長さ。LeadingMSとTrailingMSはKokoroの先頭と末尾の境界記号の長さ。
	PieceMS               []float64
	LeadingMS, TrailingMS float64
	// F0は出力のF0（Hz、F0FrameMSごと、0は無声）。
	F0 []float64
}

// ReferenceはKokoroの読み。
type Reference struct {
	// Audioは24kHzの音声。F0はKokoroの内部のF0（12.5msごと、Hz）。
	Audio []float32
	F0    []float32
	// PieceMSは音素列の断片ごとの長さ。LeadingMSとTrailingMSは先頭と末尾の境界記号の長さ。
	PieceMS               []float64
	LeadingMS, TrailingMS float64
}

// ReferenceF0は参照のF0を、有声区間を約20msでなめらかにしてF0FrameMSごとにする（0は無声）。
func (r Reference) ReferenceF0() []float64 {
	frames := len(r.Audio) * 1000 / kokoroRate / int(refmatch.FrameMS)
	return refmatch.ReferenceF0(r.F0, kokoroF0FrameMS, frames)
}

// Readは音素列をKokoroで読む。
func (e *Engine) Read(pieces []Piece, speed float32) (Reference, error) {
	var phonemes strings.Builder
	for _, piece := range pieces {
		phonemes.WriteString(piece.Symbols)
	}
	if speed <= 0 {
		speed = 1
	}
	result, err := e.kokoro.Synthesize(phonemes.String(), e.voice, speed, torchrng.New(kokoroSeed))
	if err != nil {
		return Reference{}, err
	}
	reference := Reference{Audio: result.Audio, F0: result.F0, PieceMS: make([]float64, len(pieces))}
	durations := result.Durations
	if len(durations) > 0 {
		reference.LeadingMS = float64(durations[0]) * kokoroDurationMS
		reference.TrailingMS = float64(durations[len(durations)-1]) * kokoroDurationMS
	}
	token := 1
	for i, piece := range pieces {
		for range e.kokoro.Tokens(piece.Symbols) {
			if token < len(durations)-1 {
				reference.PieceMS[i] += float64(durations[token]) * kokoroDurationMS
			}
			token++
		}
	}
	return reference, nil
}

// Speakは音素列をKokoroで読み、その特徴量に近い音源のフレームで合成し直す。
func (e *Engine) Speak(index *refmatch.Index, world refmatch.World, pieces []Piece, options Options) (Speech, error) {
	if index.Model != FeatureModel {
		return Speech{}, fmt.Errorf("voicebank index uses %q, want %q", index.Model, FeatureModel)
	}
	reference, err := e.Read(pieces, options.Speed)
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
	audio, err := index.Synthesize(world, refmatch.Request{
		Features: features, Frames: frames, ReferenceF0: referenceF0,
		CenterHz: options.CenterHz, PitchScale: options.PitchScale,
	})
	if err != nil {
		return Speech{}, err
	}
	return Speech{Audio: audio, SampleRate: index.SampleRate, PieceMS: reference.PieceMS,
		LeadingMS: reference.LeadingMS, TrailingMS: reference.TrailingMS,
		F0: index.OutputF0(referenceF0, options.CenterHz, options.PitchScale)}, nil
}
