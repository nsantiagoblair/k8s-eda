package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/handler"
	"github.com/nsantiagoblair/k8s-eda/store"
)

// newMux wires up a fresh store and handler for each test, ensuring tests are
// isolated from each other.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	handler.NewTradeHandler(store.NewInMemoryStore()).RegisterRoutes(mux)
	return mux
}

// post is a small helper that fires a POST request through the mux directly,
// without starting a real network server.
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

// --- POST /trades -----------------------------------------------------------

func TestCreateTrade(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid buy",
			body:       `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "valid sell",
			body:       `{"asset":"NVDA","side":"SELL","quantity":3,"limitPrice":890.00}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "zero quantity",
			body:       `{"asset":"AAPL","side":"BUY","quantity":0,"limitPrice":195.00}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "missing asset",
			body:       `{"asset":"","side":"BUY","quantity":10,"limitPrice":195.00}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "unknown side",
			body:       `{"asset":"AAPL","side":"HOLD","quantity":10,"limitPrice":195.00}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "malformed JSON",
			body:       `{not json`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := post(newMux(), "/trades", tt.body)

			if w.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d — body: %s", tt.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}

func TestCreateTrade_ResponseShape(t *testing.T) {
	w := post(newMux(), "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var resp struct {
		ID     string `json:"id"`
		Asset  string `json:"asset"`
		Side   string `json:"side"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}

	if resp.ID == "" {
		t.Error("expected non-empty id in response")
	}
	if resp.Asset != "AAPL" {
		t.Errorf("expected asset AAPL, got %s", resp.Asset)
	}
	if resp.Side != "BUY" {
		t.Errorf("expected side BUY, got %s", resp.Side)
	}
	if resp.Status != "PENDING" {
		t.Errorf("expected status PENDING, got %s", resp.Status)
	}
}

// --- GET /trades/{id} -------------------------------------------------------

func TestGetTradeByID(t *testing.T) {
	mux := newMux()

	// create a trade first so we have a real ID to look up
	w := post(mux, "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)
	var created struct {
		ID string `json:"id"`
	}
	json.NewDecoder(w.Body).Decode(&created) //nolint:errcheck

	t.Run("found", func(t *testing.T) {
		w := get(mux, "/trades/"+created.ID)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("not found", func(t *testing.T) {
		w := get(mux, "/trades/does-not-exist")
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404, got %d", w.Code)
		}
	})
}

// --- GET /trades ------------------------------------------------------------

func TestListTrades(t *testing.T) {
	mux := newMux()

	t.Run("empty store returns empty array", func(t *testing.T) {
		w := get(mux, "/trades")
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		// should be [] not null
		body := strings.TrimSpace(w.Body.String())
		if !bytes.HasPrefix([]byte(body), []byte("[")) {
			t.Errorf("expected JSON array, got: %s", body)
		}
	})

	t.Run("returns created trades", func(t *testing.T) {
		post(mux, "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)
		post(mux, "/trades", `{"asset":"TSLA","side":"SELL","quantity":5,"limitPrice":250.00}`)

		w := get(mux, "/trades")

		var trades []map[string]any
		json.NewDecoder(w.Body).Decode(&trades) //nolint:errcheck

		if len(trades) != 2 {
			t.Errorf("expected 2 trades, got %d", len(trades))
		}
	})
}

// --- Content-Type -----------------------------------------------------------

func TestContentTypeIsJSON(t *testing.T) {
	w := post(newMux(), "/trades", `{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}`)

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", ct)
	}
}
