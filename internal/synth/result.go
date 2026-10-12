package synth

import (
	"fmt"
	"strings"

	"github.com/yh2237/UtauTTS/internal/audio"
	"github.com/yh2237/UtauTTS/internal/label"
	"github.com/yh2237/UtauTTS/internal/render"
	"github.com/yh2237/UtauTTS/internal/sidecar"
	"github.com/yh2237/UtauTTS/internal/tts"
)

type Result struct {
	*tts.Result
	RendererID string
	DurationMS float64
	Lab        string
}

func NewResult(result *tts.Result, rendererID string) (*Result, error) {
	if result == nil || result.Audio == nil {
		return nil, fmt.Errorf("synthesis result contains no audio")
	}
	durationMS := float64(len(result.Audio.Data)) * 1000 / float64(result.Audio.SampleRate)
	lab, err := label.HTS(result.RenderedPlan(), result.MoraDurationsMS, durationMS)
	if err != nil {
		return nil, fmt.Errorf("build phoneme label: %w", err)
	}
	return &Result{Result: result, RendererID: rendererID, DurationMS: durationMS, Lab: lab}, nil
}

// DiagnosticsはRendererの診断と計画上の欠落音素をまとめる。
func (r *Result) Diagnostics() []render.RenderDiagnostic {
	diagnostics := make([]render.RenderDiagnostic, 0)
	if r.RenderReport != nil {
		diagnostics = append(diagnostics, r.RenderReport.Diagnostics...)
	}
	for _, gap := range r.Plan.MissingPhones {
		diagnostics = append(diagnostics, render.RenderDiagnostic{
			Severity: "warning", Code: "missing-phones",
			Message: fmt.Sprintf("position %d: %s", gap.Position, strings.Join(gap.Phones, " ")),
		})
	}
	return diagnostics
}

// ReportはPCMを含まないCLI・HTTP共通の結果情報。
type Report struct {
	Reading       string                    `json:"reading"`
	RendererID    string                    `json:"renderer_id"`
	DurationMS    float64                   `json:"duration_ms"`
	SampleRate    int                       `json:"sample_rate"`
	Channels      int                       `json:"channels"`
	UnitCount     int                       `json:"unit_count"`
	PhonemeLabels string                    `json:"phoneme_labels"`
	Diagnostics   []render.RenderDiagnostic `json:"diagnostics"`
}

func (r *Result) Report() Report {
	return Report{
		Reading: r.Plan.Reading, RendererID: r.RendererID, DurationMS: r.DurationMS,
		SampleRate: r.Audio.SampleRate, Channels: r.Audio.Channels, UnitCount: len(r.Plan.Units),
		PhonemeLabels: r.Lab, Diagnostics: r.Diagnostics(),
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
