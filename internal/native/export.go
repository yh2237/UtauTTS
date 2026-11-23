package native

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yh2237/UtauTTS/internal/aviutl"
	"github.com/yh2237/UtauTTS/internal/openutau"
	"github.com/yh2237/UtauTTS/internal/synth"
)

func (e *Engine) writeSidecars(data []byte) (any, error) {
	var request struct {
		WAVPath   string `json:"wav_path"`
		Text      string `json:"text"`
		Lab       string `json:"lab"`
		Encoding  string `json:"encoding"`
		WriteText bool   `json:"write_text"`
		WriteLab  bool   `json:"write_lab"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode sidecar request: %w", err)
	}
	if request.WAVPath == "" {
		return nil, fmt.Errorf("wav_path is required")
	}
	if err := synth.WriteSidecars(request.WAVPath, synth.ExportOptions{
		WriteText: request.WriteText, WriteLab: request.WriteLab,
		TextEncoding: request.Encoding, Text: request.Text,
	}, request.Lab); err != nil {
		return nil, err
	}
	return map[string]any{"status": "ok"}, nil
}

func (e *Engine) writeExo(data []byte) (any, error) {
	var request struct {
		OutputPath string   `json:"output_path"`
		Files      []string `json:"files"`
		FrameRate  int      `json:"frame_rate"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode write exo request: %w", err)
	}
	if request.OutputPath == "" {
		return nil, fmt.Errorf("output_path is required")
	}
	if len(request.Files) == 0 {
		return nil, fmt.Errorf("files are required")
	}
	if request.FrameRate <= 0 {
		request.FrameRate = 60
	}
	for _, file := range request.Files {
		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("WAV file not found: %s", file)
		}
	}
	outputPath, err := filepath.Abs(request.OutputPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}
	if err := aviutl.WriteExo(outputPath, request.Files, request.FrameRate); err != nil {
		return nil, err
	}
	return map[string]any{"exo_path": outputPath}, nil
}

func (e *Engine) exportUstx(data []byte) (any, error) {
	var request struct {
		OutputPath string          `json:"output_path"`
		Project    json.RawMessage `json:"project"`
	}
	if err := json.Unmarshal(data, &request); err != nil {
		return nil, fmt.Errorf("decode export ustx request: %w", err)
	}
	if request.OutputPath == "" {
		return nil, fmt.Errorf("output_path is required")
	}
	if len(request.Project) == 0 {
		return nil, fmt.Errorf("project is required")
	}
	project, err := openutau.ParseUtauTTSProject(request.Project)
	if err != nil {
		return nil, err
	}
	output, err := e.synth.ExportUSTX(e.ctx, project)
	if err != nil {
		return nil, err
	}
	outputPath, err := filepath.Abs(request.OutputPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(outputPath, output, 0o644); err != nil {
		return nil, err
	}
	return map[string]any{"ustx_path": outputPath}, nil
}
