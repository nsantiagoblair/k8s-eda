// Package handler contains the HTTP handlers for the trade API.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/nsantiagoblair/k8s-eda/broker"
	"github.com/nsantiagoblair/k8s-eda/event"
	"github.com/nsantiagoblair/k8s-eda/trade"
)

// TradeHandler handles HTTP requests for trade operations.
type TradeHandler struct {
	store     trade.Store
	publisher broker.Publisher
}

func NewTradeHandler(store trade.Store, publisher broker.Publisher) *TradeHandler {
	return &TradeHandler{store: store, publisher: publisher}
}

// RegisterRoutes wires up the trade endpoints on the provided mux.
func (h *TradeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /trades", h.create)
	mux.HandleFunc("GET /trades/{id}", h.getByID)
	mux.HandleFunc("GET /trades", h.list)
	mux.HandleFunc("POST /trades/{id}/submit", h.submit)
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

// POST /trades/{id}/submit
//
// Transitions the trade to SUBMITTED and publishes a TradeSubmitted event.
// Returns 202 Accepted immediately — the final outcome (FULFILLED/REJECTED)
// arrives asynchronously via the trade-fulfilled and trade-rejected topics.
// Poll GET /trades/{id} to check the current status.
func (h *TradeHandler) submit(w http.ResponseWriter, r *http.Request) {
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

	if err := t.Transition(trade.Submitted); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err := h.store.Save(t); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save trade")
		return
	}

	evt := event.TradeSubmitted{
		TradeID:    t.ID,
		Asset:      t.Asset,
		Side:       t.Side.String(),
		Quantity:   t.Quantity,
		LimitPrice: t.LimitPrice,
		OccurredAt: time.Now(),
	}
	if err := h.publisher.Publish(r.Context(), event.TopicTradeSubmitted, evt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to publish event")
		return
	}

	// 202 Accepted: the request has been accepted for processing but is not
	// yet complete. The client should poll GET /trades/{id} for the outcome.
	writeJSON(w, http.StatusAccepted, t)
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
