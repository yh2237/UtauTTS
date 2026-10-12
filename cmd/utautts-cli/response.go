package main

import (
	"context"
	"errors"

	"github.com/yh2237/UtauTTS/internal/synth"
)

type cliResponse struct {
	OK      bool                `json:"ok"`
	Result  *cliSynthesisReport `json:"result,omitempty"`
	Version *cliVersionReport   `json:"version,omitempty"`
	Catalog *cliCatalogReport   `json:"catalog,omitempty"`
	Doctor  *cliDoctorReport    `json:"doctor,omitempty"`
	Error   *cliErrorReport     `json:"error,omitempty"`
}

type cliSynthesisReport struct {
	Path string `json:"path"`
	synth.Report
}

type cliVersionReport struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type cliErrorReport struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type cliError struct {
	code string
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func cliFailure(code string, err error) error {
	if err == nil {
		return nil
	}
	return &cliError{code: code, err: err}
}

func cliErrorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline_exceeded"
	case errors.Is(err, synth.ErrUnavailable):
		return "unavailable"
	}
	var classified *cliError
	if errors.As(err, &classified) {
		return classified.code
	}
	return "synthesis_error"
}

func cliExitCode(err error) int {
	if err == nil {
		return 0
	}
	if cliErrorCode(err) == "flag_parse_error" {
		return 2
	}
	// 既存の終了コードを保ち、詳細はJSONで区別する。
	return 1
}

func synthesisReport(path string, output *synth.Result) *cliSynthesisReport {
	return &cliSynthesisReport{Path: path, Report: output.Report()}
}
