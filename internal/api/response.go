package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/synth"
)

// エラーコードの語彙。openapi.yamlのError.codeとdocs/server.mdの表はこの定数に合わせる。
const (
	codeInvalidRequest   = "invalid_request"
	codeLimitExceeded    = "limit_exceeded"
	codeUnavailable      = "unavailable"
	codeSynthesisError   = "synthesis_error"
	codeCanceled         = "canceled"
	codeDeadlineExceeded = "deadline_exceeded"
	codeForbidden        = "forbidden"
	codeInternalError    = "internal_error"
	codeNotFound         = "not_found"
	codeMethodNotAllowed = "method_not_allowed"
	codeTooManyRequests  = "too_many_requests"
	codeNotReady         = "not_ready"
)

// cliErrorCodeAliasesは、CLIのエラーコードをHTTPのエラーコードへ対応づける。
// CLI側（cmd/utautts-cli/response.go）の語彙を変えずに、同じ失敗を同じ名前で扱えるようにする。
// 共通の定義へ寄せるときはこの表を正とする。
var cliErrorCodeAliases = map[string]string{
	"invalid_arguments":    codeInvalidRequest,
	"flag_parse_error":     codeInvalidRequest,
	"input_error":          codeInvalidRequest,
	"initialization_error": codeUnavailable,
	"shutdown_error":       codeInternalError,
	"output_error":         codeInternalError,
}

// ErrorCodeForCLIはCLIのエラーコードに対応するHTTPのエラーコードを返す。
// 対応表にないコードはそのまま返す。
func ErrorCodeForCLI(code string) string {
	if mapped, ok := cliErrorCodeAliases[code]; ok {
		return mapped
	}
	return code
}

// 既存クライアント向けにerrorは文字列のまま保ち、判定用のcodeを添える。
// fieldとexpectedは入力不正のときだけ付ける。
type errorResponse struct {
	Error    string `json:"error"`
	Code     string `json:"code"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
}

// inputErrorは、入力不正の項目名と期待する形式を添える。
type inputError struct {
	field    string
	expected string
	err      error
}

func (e *inputError) Error() string { return e.err.Error() }
func (e *inputError) Unwrap() error { return e.err }

// invalidInputは、項目名と期待する形式を持つ入力不正のエラーを作る。
func invalidInput(field, expected, message string) error {
	return &inputError{field: field, expected: expected, err: errors.New(message)}
}

func errorCodeForStatus(status int) string {
	switch status {
	case http.StatusBadRequest:
		return codeInvalidRequest
	case http.StatusForbidden:
		return codeForbidden
	case http.StatusNotFound:
		return codeNotFound
	case http.StatusMethodNotAllowed:
		return codeMethodNotAllowed
	case http.StatusConflict:
		return codeNotReady
	case http.StatusRequestEntityTooLarge:
		return codeLimitExceeded
	case http.StatusRequestTimeout:
		return codeCanceled
	case http.StatusTooManyRequests:
		return codeTooManyRequests
	case http.StatusUnprocessableEntity:
		return codeSynthesisError
	case http.StatusServiceUnavailable:
		return codeUnavailable
	}
	return codeInternalError
}

// apiErrorは、エラーのHTTPステータスと応答本文を組み立てる。
func apiError(status int, err error) (int, errorResponse) {
	code := errorCodeForStatus(status)
	switch {
	case errors.Is(err, context.Canceled):
		status, code = http.StatusRequestTimeout, codeCanceled
	case errors.Is(err, context.DeadlineExceeded):
		status, code = http.StatusRequestTimeout, codeDeadlineExceeded
	case errors.Is(err, lifecycle.ErrClosed):
		status, code = http.StatusServiceUnavailable, codeUnavailable
	case errors.Is(err, synth.ErrUnavailable):
		code = codeUnavailable
	}
	// HTTPの応答は常にHTTPの語彙で返す。CLIの語彙が混ざっても対応表で寄せる。
	response := errorResponse{Error: err.Error(), Code: ErrorCodeForCLI(code)}
	var input *inputError
	if errors.As(err, &input) {
		response.Field, response.Expected = input.field, input.expected
	}
	return status, response
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	status, response := apiError(status, err)
	if status == http.StatusTooManyRequests {
		w.Header().Set("Retry-After", "5")
	}
	writeJSON(w, status, response)
}
