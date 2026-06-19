// Chapter 2 — HTTP API
//
// Wires up the trade domain behind a simple HTTP server.
// The domain model from Chapter 1 is unchanged; we're adding a delivery
// mechanism (HTTP) on top of it.
package main

import (
	"log"
	"net/http"

	"github.com/nsantiagoblair/k8s-eda/handler"
	"github.com/nsantiagoblair/k8s-eda/trade"
)

func main() {
	store := trade.NewInMemoryStore()

	mux := http.NewServeMux()
	handler.NewTradeHandler(store).RegisterRoutes(mux)

	log.Println("server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
