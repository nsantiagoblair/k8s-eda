// Chapter 3 — Persistence
//
// Swaps the in-memory store for a SQLite-backed store.
// The handler is completely unchanged — it only depends on the trade.Store
// interface, so it doesn't care what's underneath.
package main

import (
	"log"
	"net/http"

	"github.com/nsantiagoblair/k8s-eda/handler"
	"github.com/nsantiagoblair/k8s-eda/store"
)

func main() {
	s, err := store.NewSQLiteStore("trades.db")
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	mux := http.NewServeMux()
	handler.NewTradeHandler(s).RegisterRoutes(mux)

	log.Println("server listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
