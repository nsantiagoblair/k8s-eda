package provider_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nsantiagoblair/k8s-eda/provider"
)

// newStubServer starts a test HTTP server that responds with the given response.
func newStubServer(t *testing.T, resp provider.SubmitResponse, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/orders" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestClient_Submit_Fulfilled(t *testing.T) {
	srv := newStubServer(t, provider.SubmitResponse{
		TradeID:     "trade-1",
		Status:      "FULFILLED",
		FilledPrice: 195.00,
		FilledAt:    time.Now(),
	}, http.StatusOK)

	client := provider.NewClient(srv.URL)
	resp, err := client.Submit(context.Background(), provider.SubmitRequest{
		TradeID:    "trade-1",
		Asset:      "AAPL",
		Side:       "BUY",
		Quantity:   10,
		LimitPrice: 195.00,
	})

	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}
	if resp.Status != "FULFILLED" {
		t.Errorf("expected FULFILLED, got %s", resp.Status)
	}
	if resp.FilledPrice != 195.00 {
		t.Errorf("expected FilledPrice 195.00, got %.2f", resp.FilledPrice)
	}
}

func TestClient_Submit_Rejected(t *testing.T) {
	srv := newStubServer(t, provider.SubmitResponse{
		TradeID: "trade-2",
		Status:  "REJECTED",
		Reason:  "price below threshold",
	}, http.StatusOK)

	client := provider.NewClient(srv.URL)
	resp, err := client.Submit(context.Background(), provider.SubmitRequest{
		TradeID:    "trade-2",
		Asset:      "AAPL",
		Side:       "BUY",
		Quantity:   10,
		LimitPrice: 1.00,
	})

	if err != nil {
		t.Fatalf("Submit() unexpected error: %v", err)
	}
	if resp.Status != "REJECTED" {
		t.Errorf("expected REJECTED, got %s", resp.Status)
	}
	if resp.Reason == "" {
		t.Error("expected non-empty rejection reason")
	}
}

func TestClient_Submit_ProviderError(t *testing.T) {
	srv := newStubServer(t, provider.SubmitResponse{}, http.StatusInternalServerError)

	client := provider.NewClient(srv.URL)
	_, err := client.Submit(context.Background(), provider.SubmitRequest{
		TradeID: "trade-3",
		Asset:   "AAPL",
	})

	if err == nil {
		t.Error("expected error on non-200 response, got nil")
	}
}

func TestClient_Submit_ContextCancellation(t *testing.T) {
	// Server that hangs to allow cancellation to fire.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // block until cancelled
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	client := provider.NewClient(srv.URL)
	_, err := client.Submit(ctx, provider.SubmitRequest{TradeID: "trade-4"})

	if err == nil {
		t.Error("expected error on cancelled context, got nil")
	}
}
