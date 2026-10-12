package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/yh2237/UtauTTS/internal/lifecycle"
	"github.com/yh2237/UtauTTS/internal/synth"
)

// Keep error as a string for existing clients; code is the machine-facing field.
type errorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	code := "internal_error"
	switch {
	case errors.Is(err, context.Canceled):
		status, code = http.StatusRequestTimeout, "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		status, code = http.StatusRequestTimeout, "deadline_exceeded"
	case errors.Is(err, lifecycle.ErrClosed):
		status, code = http.StatusServiceUnavailable, "unavailable"
	case errors.Is(err, synth.ErrUnavailable):
		code = "unavailable"
	case status == http.StatusBadRequest:
		code = "invalid_request"
	case status == http.StatusRequestEntityTooLarge:
		code = "limit_exceeded"
	case status == http.StatusForbidden:
		code = "forbidden"
	case status == http.StatusUnprocessableEntity:
		code = "synthesis_error"
	case status == http.StatusServiceUnavailable:
		code = "unavailable"
	}
	writeJSON(w, status, errorResponse{Error: err.Error(), Code: code})
}
