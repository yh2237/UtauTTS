package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"

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
	// Hintは次に取る手順。無い場合は省略する。
	Hint string `json:"hint,omitempty"`
}

type cliError struct {
	code string
	hint string
	err  error
}

func (e *cliError) Error() string { return e.err.Error() }
func (e *cliError) Unwrap() error { return e.err }

func cliFailure(code string, err error) error {
	return cliFailureHint(code, "", err)
}

// cliFailureHintは次に取る手順を添えた失敗を返す。
func cliFailureHint(code, hint string, err error) error {
	if err == nil {
		return nil
	}
	return &cliError{code: code, hint: hint, err: err}
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

// cliErrorHintは失敗の対処を1行で返す。個別の指定が無ければコードごとの既定を使う。
func cliErrorHint(err error) string {
	var classified *cliError
	if errors.As(err, &classified) && classified.hint != "" {
		return classified.hint
	}
	return defaultHints[cliErrorCode(err)]
}

// defaultHintsはコードごとの既定の対処。
var defaultHints = map[string]string{
	"flag_parse_error":     "run utautts-cli --help to see the flags",
	"invalid_arguments":    "run utautts-cli --help to see the flags",
	"input_error":          "check the input file and its encoding (UTF-8)",
	"unavailable":          "check the voicebank, model and Open JTalk paths with --doctor",
	"initialization_error": "check the runtime directory and the Open JTalk paths with --doctor",
	"synthesis_error":      "check the voicebank and the reading; --doctor reports missing resources",
	"canceled":             "run again if the interruption was unintended",
	"deadline_exceeded":    "raise --timeout, or use 0 to disable it",
	"output_error":         "check that the output directory exists and is writable",
}

// 終了コード。docs/cli.mdの表と一致させる。
const (
	exitOK        = 0
	exitInternal  = 1
	exitUsage     = 2
	exitInput     = 3
	exitResource  = 4
	exitSynthesis = 5
	exitOutput    = 6
)

// exitCodesはエラーコードごとの終了コード。
var exitCodes = map[string]int{
	"flag_parse_error":     exitUsage,
	"invalid_arguments":    exitUsage,
	"input_error":          exitInput,
	"unavailable":          exitResource,
	"initialization_error": exitResource,
	"synthesis_error":      exitSynthesis,
	"canceled":             exitSynthesis,
	"deadline_exceeded":    exitSynthesis,
	"output_error":         exitOutput,
	"shutdown_error":       exitInternal,
}

func cliExitCode(err error) int {
	if err == nil {
		return exitOK
	}
	if code, ok := exitCodes[cliErrorCode(err)]; ok {
		return code
	}
	return exitInternal
}

func errorReport(err error) *cliErrorReport {
	return &cliErrorReport{Code: cliErrorCode(err), Message: err.Error(), Hint: cliErrorHint(err)}
}

func writeJSONResponse(output io.Writer, report cliResponse) error {
	encoder := json.NewEncoder(output)
	// ヒントの<...>を読みやすくするため、HTML向けのエスケープはしない。
	encoder.SetEscapeHTML(false)
	return encoder.Encode(report)
}

func synthesisReport(path string, output *synth.Result) *cliSynthesisReport {
	return &cliSynthesisReport{Path: path, Report: output.Report()}
}
