// Chapter 4 — Provider Stub
//
// The trade API now calls an external provider to execute trades.
// Run the provider stub first (cmd/provider-stub), then this service.
package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/nsantiagoblair/k8s-eda/handler"
	"github.com/nsantiagoblair/k8s-eda/provider"
	"github.com/nsantiagoblair/k8s-eda/store"
)

func main() {
	addr        := flag.String("addr", ":8080", "address to listen on")
	providerURL := flag.String("provider-url", "http://localhost:9090", "provider stub base URL")
	dbPath      := flag.String("db", "trades.db", "path to SQLite database file")
	flag.Parse()

	s, err := store.NewSQLiteStore(*dbPath)
	if err != nil {
		log.Fatalf("failed to open store: %v", err)
	}
	defer s.Close()

	p := provider.NewClient(*providerURL)

	mux := http.NewServeMux()
	handler.NewTradeHandler(s, p).RegisterRoutes(mux)

	log.Printf("trade-api listening on %s (provider: %s)", *addr, *providerURL)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
