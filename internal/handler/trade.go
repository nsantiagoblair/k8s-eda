// Package handler contains the HTTP handlers for the trade API.
// Handlers are responsible only for parsing HTTP input, calling the service,
// mapping domain errors to HTTP status codes, and writing the response.
// All business logic lives in the service layer.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nsantiagoblair/k8s-eda/internal/store"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

// tradeService is the interface the handler depends on.
// Defined here (the consumer) rather than in the service package — a Go idiom
// that keeps packages decoupled and makes the handler's exact requirements clear.
type tradeService interface {
	Create(asset string, side trade.Side, qty int, price float64) (*trade.Trade, error)
	Submit(ctx context.Context, id string) (*trade.Trade, error)
	GetByID(id string) (*trade.Trade, error)
	List() ([]*trade.Trade, error)
}

// TradeHandler handles HTTP requests for trade operations.
type TradeHandler struct {
	service tradeService
}

func NewTradeHandler(svc tradeService) *TradeHandler {
	return &TradeHandler{service: svc}
}

func (h *TradeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /trades", h.create)
	mux.HandleFunc("GET /trades/{id}", h.getByID)
	mux.HandleFunc("GET /trades", h.list)
	mux.HandleFunc("POST /trades/{id}/submit", h.submit)
}

type createRequest struct {
	Asset      string     `json:"asset"`
	Side       trade.Side `json:"side"`
	Quantity   int        `json:"quantity"`
	LimitPrice float64    `json:"limitPrice"`
}

func (h *TradeHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	t, err := h.service.Create(req.Asset, req.Side, req.Quantity, req.LimitPrice)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (h *TradeHandler) getByID(w http.ResponseWriter, r *http.Request) {
	t, err := h.service.GetByID(r.PathValue("id"))
	if err != nil {
		writeError(w, httpStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *TradeHandler) list(w http.ResponseWriter, r *http.Request) {
	trades, err := h.service.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list trades")
		return
	}
	writeJSON(w, http.StatusOK, trades)
}

func (h *TradeHandler) submit(w http.ResponseWriter, r *http.Request) {
	t, err := h.service.Submit(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, httpStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, t)
}

// httpStatus maps domain errors to HTTP status codes.
// All other errors map to 500.
func httpStatus(err error) int {
	var notFound *store.ErrNotFound
	if errors.As(err, &notFound) {
		return http.StatusNotFound
	}

	var invalidTransition *trade.ErrInvalidTransition
	if errors.As(err, &invalidTransition) {
		return http.StatusConflict
	}

	return http.StatusInternalServerError
}

// --- helpers ----------------------------------------------------------------

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
