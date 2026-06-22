// Chapter 5 — Events
//
// The trade API now communicates with the provider via Kafka events instead
// of synchronous HTTP calls. POST /trades/{id}/submit publishes a TradeSubmitted
// event and returns 202 Accepted. Background consumers update the trade's status
// when TradeFulfilled or TradeRejected events arrive.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nsantiagoblair/k8s-eda/broker"
	"github.com/nsantiagoblair/k8s-eda/consumer"
	"github.com/nsantiagoblair/k8s-eda/event"
	"github.com/nsantiagoblair/k8s-eda/handler"
	"github.com/nsantiagoblair/k8s-eda/store"
)

func main() {
	addr    := flag.String("addr",    ":8080",           "address to listen on")
	brokers := flag.String("brokers", "localhost:9092",  "comma-separated Kafka broker addresses")
	dbPath  := flag.String("db",      "trades.db",       "path to SQLite database file")
	flag.Parse()

	// Graceful shutdown: ctx is cancelled when SIGINT or SIGTERM is received.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := store.NewSQLiteStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	pub := broker.NewKafkaPublisher([]string{*brokers})
	defer pub.Close()

	// Wire up the HTTP handler.
	mux := http.NewServeMux()
	handler.NewTradeHandler(s, pub).RegisterRoutes(mux)

	// Start a consumer for each result topic in its own goroutine.
	tc := consumer.NewTradeConsumer(s)
	startConsumer(ctx, *brokers, event.TopicTradeFulfilled, "trade-api", tc.HandleFulfilled)
	startConsumer(ctx, *brokers, event.TopicTradeRejected,  "trade-api", tc.HandleRejected)

	// Start HTTP server — using http.Server directly so we can shut it down gracefully.
	srv := &http.Server{Addr: *addr, Handler: mux}
	go func() {
		log.Printf("trade-api listening on %s", *addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	// Block until shutdown signal.
	<-ctx.Done()
	log.Println("shutting down trade-api...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx) //nolint:errcheck
}

func startConsumer(ctx context.Context, brokerAddr, topic, groupID string, handler broker.Handler) {
	c := broker.NewKafkaConsumer([]string{brokerAddr}, topic, groupID)
	go func() {
		defer c.Close()
		if err := c.Run(ctx, handler); err != nil {
			log.Printf("consumer %s stopped: %v", topic, err)
		}
	}()
}
