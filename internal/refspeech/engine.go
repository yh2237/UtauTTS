package refspeech

import (
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/yh2237/UtauTTS/internal/hubert"
	"github.com/yh2237/UtauTTS/internal/kokoro"
	"github.com/yh2237/UtauTTS/internal/refmatch"
	"github.com/yh2237/gograd/dsp"
	"github.com/yh2237/gograd/torchrng"
)

const (
	// FeatureModelは索引の特徴量の種類。変えたら索引を作り直す。
	FeatureModel = "hubert-base-ls960-layer6"
	hubertLayer  = 6
	hubertRate   = 16000
	kokoroRate   = 24000
	// DurationUnitMSはKokoroの音素長の単位（600サンプル）。
	DurationUnitMS = 25.0
	// kokoroSamplesPerTokenは長さ1あたりのKokoroの出力のサンプル数。
	kokoroSamplesPerToken = 600
	kokoroF0FrameMS       = 12.5
	kokoroSeed            = 0
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
	reads   []*cachedRead
}

type cachedRead struct {
	key       string
	reference Reference
	prosody   *kokoro.Prosody
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
	// Audioは24kHzの音声（ReadProsodyではnil）。F0はKokoroの内部のF0（12.5msごと、Hz）。
	Audio []float32
	F0    []float32
	Timing
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

// Readは音素列をKokoroで読む。durationsを渡すと、その長さで読む。返す値は書き換えない。
func (e *Engine) Read(pieces []Piece, speed float32, durations []int) (Reference, error) {
	return e.read(pieces, speed, durations, true)
}

// ReadProsodyはReadと同じく読むが、波形は作らずに記号ごとの長さとF0だけを返す。
// 後で同じ引数でReadを呼ぶと、ここで求めた結果から波形だけを作る。
func (e *Engine) ReadProsody(pieces []Piece, speed float32, durations []int) (Reference, error) {
	return e.read(pieces, speed, durations, false)
}

func (e *Engine) read(pieces []Piece, speed float32, durations []int, withAudio bool) (Reference, error) {
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
	var entry *cachedRead
	for _, cached := range e.reads {
		if cached.key == key {
			entry = cached
		}
	}
	e.readsMu.Unlock()
	if entry == nil {
		prosody, err := e.kokoro.ReadProsody(phonemes.String(), e.voice, speed, durations)
		if err != nil {
			return Reference{}, err
		}
		entry = &cachedRead{key: key, prosody: prosody, reference: Reference{F0: prosody.F0,
			Timing: Timing{Durations: prosody.Durations, PieceTokens: make([][2]int, len(pieces))}}}
		token := 1
		for i, piece := range pieces {
			start := token
			token += len(e.kokoro.Tokens(piece.Symbols))
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
	audio, err := e.kokoro.Decode(entry.prosody, torchrng.New(kokoroSeed))
	if err != nil {
		return Reference{}, err
	}
	e.readsMu.Lock()
	entry.reference.Audio = audio
	reference = entry.reference
	e.readsMu.Unlock()
	return reference, nil
}

// Speakは音素列をKokoroで読み、その特徴量に近い音源のフレームで合成し直す。
// phonesを渡すと、各フレームでその音素を含む別名の録音だけを候補にする。
func (e *Engine) Speak(index *refmatch.Index, world refmatch.World, pieces []Piece, options Options, phones map[string][]string) (Speech, error) {
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
		ReferenceLevel: refmatch.ReferenceLevels(samples, kokoroRate, frames*4),
		Candidates:     e.candidates(index, reference, pieces, phones, frames),
	})
	if err != nil {
		return Speech{}, err
	}
	return Speech{Audio: audio, SampleRate: index.SampleRate, Timing: reference.Timing, F0: outF0, AutomaticF0: automaticF0}, nil
}

// candidatesは、各フレームでその音素を含む別名の録音だけを候補にする表を作る。
// 音素に対応する録音がないフレームと、phonesが空のときはnilを返し、索引全体から選ばせる。
func (e *Engine) candidates(index *refmatch.Index, reference Reference, pieces []Piece, phones map[string][]string, frames int) [][]int32 {
	if len(phones) == 0 || index.Frames == 0 || frames == 0 {
		return nil
	}
	fileID := make(map[string]int32, len(index.Files))
	for id, path := range index.Files {
		fileID[filepath.Clean(path)] = int32(id)
	}
	framesByFile := make([][]int32, len(index.Files))
	for frame, file := range index.File {
		framesByFile[file] = append(framesByFile[file], int32(frame))
	}
	candidateFrames := make(map[string][]int32, len(phones))
	for phone, paths := range phones {
		var candidates []int32
		for _, path := range paths {
			if id, ok := fileID[filepath.Clean(path)]; ok {
				candidates = append(candidates, framesByFile[id]...)
			}
		}
		if len(candidates) > 0 {
			candidateFrames[phone] = candidates
		}
	}
	if len(candidateFrames) == 0 {
		return nil
	}
	tokenPhones := make([]string, len(reference.Durations))
	for i, piece := range pieces {
		if i >= len(reference.PieceTokens) {
			break
		}
		start, end := reference.PieceTokens[i][0], reference.PieceTokens[i][1]
		symbols := e.kokoro.TokenSymbols(piece.Symbols)
		for k := 0; k < len(symbols) && start+k < end && start+k < len(tokenPhones); k++ {
			tokenPhones[start+k] = kokoroARPAbet(symbols[k])
		}
	}
	starts := make([]float64, len(reference.Durations)+1)
	for i, duration := range reference.Durations {
		starts[i+1] = starts[i] + float64(duration)*DurationUnitMS
	}
	result := make([][]int32, frames)
	for t := range frames {
		milliseconds := (float64(t) + 0.5) * refmatch.FrameMS
		token := sort.Search(len(starts), func(i int) bool { return starts[i] > milliseconds }) - 1
		if token < 0 || token >= len(tokenPhones) {
			continue
		}
		result[t] = candidateFrames[tokenPhones[token]]
	}
	return result
}
