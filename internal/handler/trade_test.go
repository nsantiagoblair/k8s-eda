package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
	"github.com/nsantiagoblair/k8s-eda/internal/event"
	"github.com/nsantiagoblair/k8s-eda/internal/handler"
	"github.com/nsantiagoblair/k8s-eda/internal/service"
	"github.com/nsantiagoblair/k8s-eda/internal/infra/memory"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

func newMux(pub broker.Publisher) *http.ServeMux {
	s := memory.NewStore(func(t *trade.Trade) string { return t.ID })
	svc := service.NewTradeService(s, pub)
	mux := http.NewServeMux()
	handler.NewTradeHandler(svc).RegisterRoutes(mux)
	return mux
}

func post(mux *http.ServeMux, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func get(mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func TestCreateTrade(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "valid buy",      body: `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`, wantStatus: http.StatusCreated},
		{name: "valid sell",     body: `{"asset":"NVDA","side":"SELL","quantity":3,"limitPrice":890.00}`, wantStatus: http.StatusCreated},
		{name: "zero quantity",  body: `{"asset":"AAPL","side":"BUY","quantity":0,"limitPrice":195.00}`,  wantStatus: http.StatusBadRequest},
		{name: "missing asset",  body: `{"asset":"","side":"BUY","quantity":10,"limitPrice":195.00}`,     wantStatus: http.StatusBadRequest},
		{name: "unknown side",   body: `{"asset":"AAPL","side":"HOLD","quantity":10,"limitPrice":195.00}`,wantStatus: http.StatusBadRequest},
		{name: "malformed JSON", body: `{not json`,                                                        wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := post(newMux(memory.NewPublisher()), "/trades", tt.body)
			if w.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d — %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateTrade_ResponseShape(t *testing.T) {
	w := post(newMux(memory.NewPublisher()), "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)
	if w.Code != http.StatusCreated { t.Fatalf("expected 201, got %d", w.Code) }

	var resp struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Side   string `json:"side"`
	}
	json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
	if resp.ID == ""            { t.Error("expected non-empty id") }
	if resp.Status != "PENDING" { t.Errorf("Status = %s; want PENDING", resp.Status) }
	if resp.Side != "BUY"       { t.Errorf("Side = %s; want BUY", resp.Side) }
}

func TestGetTradeByID(t *testing.T) {
	mux := newMux(memory.NewPublisher())
	w := post(mux, "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)
	var created struct{ ID string `json:"id"` }
	json.NewDecoder(w.Body).Decode(&created) //nolint:errcheck

	t.Run("found",     func(t *testing.T) {
		if get(mux, "/trades/"+created.ID).Code != http.StatusOK { t.Error("expected 200") }
	})
	t.Run("not found", func(t *testing.T) {
		if get(mux, "/trades/no-such-id").Code != http.StatusNotFound { t.Error("expected 404") }
	})
}

func TestListTrades(t *testing.T) {
	mux := newMux(memory.NewPublisher())

	w := get(mux, "/trades")
	if w.Code != http.StatusOK { t.Fatalf("expected 200, got %d", w.Code) }
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("[")) {
		t.Errorf("expected JSON array, got: %s", w.Body.String())
	}

	post(mux, "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)
	post(mux, "/trades", `{"asset":"TSLA","side":"SELL","quantity":5,"limitPrice":250.00}`)

	var trades []map[string]any
	json.NewDecoder(get(mux, "/trades").Body).Decode(&trades) //nolint:errcheck
	if len(trades) != 2 { t.Errorf("expected 2 trades, got %d", len(trades)) }
}

func TestSubmitTrade(t *testing.T) {
	createTrade := func(mux *http.ServeMux) string {
		w := post(mux, "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)
		var r struct{ ID string `json:"id"` }
		json.NewDecoder(w.Body).Decode(&r) //nolint:errcheck
		return r.ID
	}

	t.Run("returns 202 and publishes event", func(t *testing.T) {
		pub := memory.NewPublisher()
		mux := newMux(pub)
		id := createTrade(mux)

		w := post(mux, "/trades/"+id+"/submit", "")
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d — %s", w.Code, w.Body.String())
		}

		var resp struct{ Status string `json:"status"` }
		json.NewDecoder(w.Body).Decode(&resp) //nolint:errcheck
		if resp.Status != "SUBMITTED" {
			t.Errorf("Status = %s; want SUBMITTED", resp.Status)
		}
		if pub.Count(event.TopicTradeSubmitted) != 1 {
			t.Errorf("expected 1 event, got %d", pub.Count(event.TopicTradeSubmitted))
		}
	})

	t.Run("trade not found returns 404", func(t *testing.T) {
		w := post(newMux(memory.NewPublisher()), "/trades/no-such-id/submit", "")
		if w.Code != http.StatusNotFound { t.Errorf("expected 404, got %d", w.Code) }
	})

	t.Run("already submitted returns 409", func(t *testing.T) {
		mux := newMux(memory.NewPublisher())
		id := createTrade(mux)
		post(mux, "/trades/"+id+"/submit", "")
		w := post(mux, "/trades/"+id+"/submit", "")
		if w.Code != http.StatusConflict { t.Errorf("expected 409, got %d", w.Code) }
	})
}

func TestContentTypeIsJSON(t *testing.T) {
	w := post(newMux(memory.NewPublisher()), "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %s; want application/json", ct)
	}
}
