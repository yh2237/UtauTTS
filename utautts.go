// Package utauttsはGUIに依存しない音声合成APIを提供する。
// 資源は初期化時に探索し、子プロセスは必要時に起動してEngine.Closeで終了する。
package utautts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/yh2237/audiodsp/wav"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/synth"
	"github.com/yh2237/UtauTTS/internal/voicebank"
)

var (
	ErrClosed         = lifecycle.ErrClosed
	ErrUnavailable    = synth.ErrUnavailable
	ErrInvalidRequest = errors.New("invalid synthesis request")
)

// Configは発話ではなく資源配置を指定する。空のパスは既定の探索規則を使う。
type Config struct {
	VoiceDirectory      string
	Renderer            string
	WorldlineBridgePath string
	OpenJTalkPath       string
	OpenJTalkDictionary string
	RendererDirectories []string
	ModelDirectories    []string
}

type Engine struct {
	runtime *synth.Runtime
	voices  *voicebank.Library
}

type voiceResolver struct{ library *voicebank.Library }

func (r voiceResolver) Resolve(id string) (string, bool) {
	summary, ok := r.library.Resolve(id)
	return summary.Path, ok
}

// Newはカタログと音源を探索する。helperの不足は必要な操作を行う際に報告する。
func New(config Config) (*Engine, error) {
	voices := voicebank.NewLibrary(config.VoiceDirectory)
	runtime, err := synth.NewRuntime(synth.RuntimeConfig{
		Renderer: config.Renderer, WorldlineBridgePath: config.WorldlineBridgePath,
		OpenJTalkPath: config.OpenJTalkPath, OpenJTalkDictionary: config.OpenJTalkDictionary,
		RendererDirectories: append([]string(nil), config.RendererDirectories...),
		ModelDirectories:    append([]string(nil), config.ModelDirectories...),
	}, voiceResolver{voices})
	if err != nil {
		return nil, err
	}
	if err := voices.Reload(); err != nil {
		return nil, errors.Join(err, runtime.Close())
	}
	return &Engine{runtime: runtime, voices: voices}, nil
}

// Closeは自分の要求をキャンセルし、完了後にキャッシュと子プロセスを解放する。
// 複数回・並行に呼んでも同じエラーを返す。
func (e *Engine) Close() error { return e.runtime.Close() }

// Reloadは要求の完了後、自分のキャッシュ・子プロセスを破棄して音源を再読込する。
// 他のEngineには影響しない。Renderer・モデルの再探索にはEngineを作り直す。
func (e *Engine) Reload() error { return e.runtime.Reload(e.voices.Reload) }

type Voicebank struct {
	ID, Name, Path, Kind string
}

func (e *Engine) Voicebanks(ctx context.Context) ([]Voicebank, error) {
	_, release, err := e.runtime.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	items := e.voices.List()
	voices := make([]Voicebank, len(items))
	for i, item := range items {
		voices[i] = Voicebank{item.ID, item.Summary.Name, item.Summary.Path, item.Summary.Kind}
	}
	return voices, nil
}

type Renderer struct {
	ID, Name  string
	Available bool
	Problems  []string
}

func (e *Engine) Renderers(ctx context.Context) ([]Renderer, error) {
	_, release, err := e.runtime.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	availability := e.runtime.Service.RendererAvailability()
	items := make([]Renderer, 0, len(e.runtime.Catalog.Renderers))
	for _, item := range e.runtime.Catalog.Renderers {
		status := availability[item.ID]
		renderer := Renderer{ID: item.ID, Name: item.DisplayName, Available: status.Available}
		for _, issue := range status.Issues {
			renderer.Problems = append(renderer.Problems, issue.Message)
		}
		items = append(items, renderer)
	}
	return items, nil
}

type Model struct {
	ID, Name, Language string
}

func (e *Engine) Models(ctx context.Context) ([]Model, error) {
	_, release, err := e.runtime.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	items := make([]Model, len(e.runtime.Catalog.Models))
	for i, item := range e.runtime.Catalog.Models {
		items[i] = Model{item.ID, item.DisplayName, item.Language}
	}
	return items, nil
}

// Requestは1発話を指定する。空のIDは既定値を使い、発音はReadingをTextより優先する。
type Request struct {
	Text, Reading, Language, Phonemizer string
	VoicebankID, RendererID, ModelID    string
	Tone, Color                         string
	Dictionary                          []DictionaryEntry
	Options                             Options
	// 共通設定はOptionsで指定し、ここには方式固有のJSONだけを渡す。
	RendererSettings map[string]json.RawMessage
}

type DictionaryEntry struct {
	Surface string `json:"surface"`
	Reading string `json:"reading"`
}

// Optionsはnilを未指定とし、0・falseも明示値として扱う。
// MoraDurationMSは正数、PauseDurationMSとReleaseMSは0以上を指定する。
type Options struct {
	MoraDurationMS, PauseDurationMS, IntonationStrength, ReleaseMS *float64
	ApplyPitch, ContextDuration, BoundaryTone, StretchAdapt        *bool
	PauseContext, EnglishWeakForm                                  *bool
}

type Analysis struct {
	Reading string
	Morae   []Mora
}

type Mora struct {
	Text, Consonant, Vowel string
	Pause                  bool
}

func (e *Engine) Analyze(ctx context.Context, request Request) (*Analysis, error) {
	ctx, release, err := e.runtime.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	input, err := request.internal()
	if err != nil {
		return nil, err
	}
	preview, err := e.runtime.Service.AnalyzeContext(ctx, input)
	if err != nil {
		return nil, err
	}
	result := &Analysis{Reading: preview.Reading, Morae: make([]Mora, len(preview.Morae))}
	for i, mora := range preview.Morae {
		result.Morae[i] = Mora{mora.Text, mora.Consonant, mora.Vowel, mora.Pause}
	}
	return result, nil
}

// Audioは呼出し側が所有する符号付き16-bit PCM。サンプルはチャンネルごとに交互に並ぶ。
type Audio struct {
	SampleRate, Channels int
	Samples              []int16
}

func (a Audio) WriteWAV(w io.Writer) error {
	return wav.Encode(w, &wav.PCM{SampleRate: a.SampleRate, Channels: a.Channels, Data: a.Samples})
}

// Phonemeの時刻は音声先頭からのms。計画上の境界であり、波形への強制アライメントではない。
type Phoneme struct {
	Symbol         string
	StartMS, EndMS float64
}

type Diagnostic struct{ Severity, Code, Message string }

type Result struct {
	Audio               Audio
	Reading, RendererID string
	DurationMS          float64
	Phonemes            []Phoneme
	Diagnostics         []Diagnostic
}

func (e *Engine) Synthesize(ctx context.Context, request Request) (*Result, error) {
	ctx, release, err := e.runtime.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	input, err := request.internal()
	if err != nil {
		return nil, err
	}
	output, err := e.runtime.Service.SynthesizeContext(ctx, input)
	if err != nil {
		return nil, err
	}
	result := &Result{
		Audio:   Audio{output.Audio.SampleRate, output.Audio.Channels, output.Audio.Data},
		Reading: output.Plan.Reading, RendererID: output.RendererID, DurationMS: output.DurationMS,
	}
	labels := strings.NewReader(output.Lab)
	for {
		var start, end int64
		var symbol string
		if _, err := fmt.Fscan(labels, &start, &end, &symbol); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode phoneme labels: %w", err)
		}
		result.Phonemes = append(result.Phonemes, Phoneme{symbol, float64(start) / 10000, float64(end) / 10000})
	}
	for _, diagnostic := range output.Diagnostics() {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{diagnostic.Severity, diagnostic.Code, diagnostic.Message})
	}
	return result, nil
}
