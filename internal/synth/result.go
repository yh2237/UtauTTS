package synth

import (
	"fmt"

	"utautts/internal/audio"
	"utautts/internal/label"
	"utautts/internal/sidecar"
	"utautts/internal/tts"
)

// Resultは各フロントエンドで共有する合成結果。
type Result struct {
	*tts.Result
	RendererID string
	DurationMS float64
	Lab        string
}

// NewResultは音声と合成計画から付随情報を生成する。
func NewResult(result *tts.Result, rendererID string) (*Result, error) {
	if result == nil || result.Audio == nil {
		return nil, fmt.Errorf("synthesis result contains no audio")
	}
	durationMS := float64(len(result.Audio.Data)) * 1000 / float64(result.Audio.SampleRate)
	lab, err := label.HTS(result.Plan, result.MoraDurationsMS, durationMS)
	if err != nil {
		return nil, fmt.Errorf("build phoneme label: %w", err)
	}
	return &Result{Result: result, RendererID: rendererID, DurationMS: durationMS, Lab: lab}, nil
}

// ExportOptionsはWAVに付随して保存するファイルを指定する。
type ExportOptions struct {
	Text         string
	WriteText    bool
	WriteLab     bool
	TextEncoding string
}

// WriteFilesはWAVと任意のTXT／LABを同名で保存する。
func WriteFiles(wavPath string, result *Result, options ExportOptions) error {
	if result == nil || result.Audio == nil {
		return fmt.Errorf("synthesis result contains no audio")
	}
	if err := audio.WriteWav(wavPath, result.Audio); err != nil {
		return err
	}
	return WriteSidecars(wavPath, options, result.Lab)
}

// WriteSidecarsは既存WAVに任意のTXT／LABを追加する。
func WriteSidecars(wavPath string, options ExportOptions, lab string) error {
	return sidecar.Write(wavPath, sidecar.Options{
		WriteText: options.WriteText,
		WriteLab:  options.WriteLab,
		Encoding:  options.TextEncoding,
		Text:      options.Text,
		Lab:       lab,
	})
}
