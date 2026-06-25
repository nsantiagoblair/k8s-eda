package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nsantiagoblair/k8s-eda/internal/order"
	"github.com/nsantiagoblair/k8s-eda/internal/store"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

// httpStatus maps domain errors to HTTP status codes.
// Shared by all handlers in this package.
func httpStatus(err error) int {
	var notFound *store.ErrNotFound
	if errors.As(err, &notFound) {
		return http.StatusNotFound
	}
	var invalidTrade *trade.ErrInvalidTransition
	if errors.As(err, &invalidTrade) {
		return http.StatusConflict
	}
	var invalidOrder *order.ErrInvalidTransition
	if errors.As(err, &invalidOrder) {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

type errorResponse struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}
