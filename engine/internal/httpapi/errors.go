package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mist941/b3s23-engine/engine/internal/engine"
)

func httpStatus(err error) int {
	switch {
	case errors.Is(err, engine.ErrRunning):
		return http.StatusConflict
	case errors.Is(err, engine.ErrClosed),
		errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}

func writeCommandError(w http.ResponseWriter, err error) {
	writeError(w, httpStatus(err), err.Error())
}
