// Provider stub — simulates an external trade execution provider.
//
// In a real system this would be a third-party service we have no control
// over. The stub lets us develop and test the trade API without a live
// integration, and will be replaced by event-driven communication in Chapter 5.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/nsantiagoblair/k8s-eda/provider"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders", handleOrder)

	log.Println("provider stub listening on :9090")
	log.Fatal(http.ListenAndServe(":9090", mux))
}

func handleOrder(w http.ResponseWriter, r *http.Request) {
	var req provider.SubmitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	log.Printf("received order: tradeID=%s asset=%s side=%s qty=%d limit=%.2f",
		req.TradeID, req.Asset, req.Side, req.Quantity, req.LimitPrice)

	resp := evaluate(req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

// evaluate applies simple stub logic to decide whether to fulfil or reject an
// order. Real providers use order books and market prices; we use a price floor.
func evaluate(req provider.SubmitRequest) provider.SubmitResponse {
	const minimumPrice = 10.00

	if req.LimitPrice < minimumPrice {
		return provider.SubmitResponse{
			TradeID: req.TradeID,
			Status:  "REJECTED",
			Reason:  "limit price below minimum accepted threshold",
		}
	}

	return provider.SubmitResponse{
		TradeID:     req.TradeID,
		Status:      "FULFILLED",
		FilledPrice: req.LimitPrice,
		FilledAt:    time.Now(),
	}
}
