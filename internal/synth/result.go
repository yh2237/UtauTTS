package synth

import (
	"fmt"

	"github.com/yh2237/UtauIDX/audio"
	"github.com/yh2237/UtauTTS/internal/label"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/sidecar"
	"github.com/yh2237/UtauTTS/internal/tts"
)

type Result struct {
	*tts.Result
	Method     string
	DurationMS float64
	Lab        string
	// Logsは合成の途中経過。索引の作成など、時間のかかる処理を伝える。
	Logs []string
	// SettingDiagnosticsは使わなかった設定。
	SettingDiagnostics []render.RenderDiagnostic
}

func NewResult(result *tts.Result, method string) (*Result, error) {
	if result == nil || result.Audio == nil {
		return nil, fmt.Errorf("synthesis result contains no audio")
	}
	durationMS := float64(len(result.Audio.Data)) * 1000 / float64(result.Audio.SampleRate)
	lab, err := label.HTS(result.RenderedPlan(), result.MoraDurationsMS, durationMS)
	if err != nil {
		return nil, fmt.Errorf("build phoneme label: %w", err)
	}
	return &Result{Result: result, Method: method, DurationMS: durationMS, Lab: lab}, nil
}

// Diagnosticsは設定と描画の診断を返す。
func (r *Result) Diagnostics() []render.RenderDiagnostic {
	diagnostics := append(make([]render.RenderDiagnostic, 0), r.SettingDiagnostics...)
	if r.RenderReport != nil {
		diagnostics = append(diagnostics, r.RenderReport.Diagnostics...)
	}
	return diagnostics
}

// ReportはPCMを含まないCLI・HTTP共通の結果情報。
type Report struct {
	Reading       string                    `json:"reading"`
	Method        string                    `json:"method"`
	DurationMS    float64                   `json:"duration_ms"`
	SampleRate    int                       `json:"sample_rate"`
	Channels      int                       `json:"channels"`
	UnitCount     int                       `json:"unit_count"`
	PhonemeLabels string                    `json:"phoneme_labels"`
	Diagnostics   []render.RenderDiagnostic `json:"diagnostics"`
	// Logsは合成の途中経過。索引の作成など、時間のかかる処理を伝える。
	Logs []string `json:"logs,omitempty"`
}

func (r *Result) Report() Report {
	return Report{
		Reading: r.Plan.Reading, Method: r.Method, DurationMS: r.DurationMS,
		SampleRate: r.Audio.SampleRate, Channels: r.Audio.Channels, UnitCount: len(r.Plan.Units),
		PhonemeLabels: r.Lab, Diagnostics: r.Diagnostics(), Logs: r.Logs,
	}
}

type ExportOptions struct {
	Text         string
	WriteText    bool
	WriteLab     bool
	TextEncoding string
}

func WriteFiles(wavPath string, result *Result, options ExportOptions) error {
	if result == nil || result.Audio == nil {
		return fmt.Errorf("synthesis result contains no audio")
	}
	if err := audio.WriteWav(wavPath, result.Audio); err != nil {
		return err
	}
	return WriteSidecars(wavPath, options, result.Lab)
}

func WriteSidecars(wavPath string, options ExportOptions, lab string) error {
	return sidecar.Write(wavPath, sidecar.Options{
		WriteText: options.WriteText,
		WriteLab:  options.WriteLab,
		Encoding:  options.TextEncoding,
		Text:      options.Text,
		Lab:       lab,
	})
}
