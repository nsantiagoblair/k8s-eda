// Package handler contains the HTTP handlers for the trade API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/nsantiagoblair/k8s-eda/trade"
)

// TradeHandler handles HTTP requests for trade operations.
// It holds a reference to a Store, injected at construction time.
type TradeHandler struct {
	store trade.Store
}

func NewTradeHandler(store trade.Store) *TradeHandler {
	return &TradeHandler{store: store}
}

// RegisterRoutes wires up the trade endpoints on the provided mux.
// Go 1.22+ supports method-prefixed patterns and {path} parameters natively.
func (h *TradeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /trades", h.create)
	mux.HandleFunc("GET /trades/{id}", h.getByID)
	mux.HandleFunc("GET /trades", h.list)
}

// createRequest is the expected JSON body for POST /trades.
type createRequest struct {
	Asset      string     `json:"asset"`
	Side       trade.Side `json:"side"`
	Quantity   int        `json:"quantity"`
	LimitPrice float64    `json:"limitPrice"`
}

// POST /trades
func (h *TradeHandler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	t, err := trade.New(req.Asset, req.Side, req.Quantity, req.LimitPrice)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.store.Save(t); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save trade")
		return
	}

	writeJSON(w, http.StatusCreated, t)
}

// GET /trades/{id}
func (h *TradeHandler) getByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	t, err := h.store.FindByID(id)
	if err != nil {
		var notFound *trade.ErrNotFound
		if errors.As(err, &notFound) {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to retrieve trade")
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// GET /trades
func (h *TradeHandler) list(w http.ResponseWriter, r *http.Request) {
	trades, err := h.store.FindAll()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list trades")
		return
	}

	writeJSON(w, http.StatusOK, trades)
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
