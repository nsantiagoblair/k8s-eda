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

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
	"github.com/nsantiagoblair/k8s-eda/internal/consumer"
	"github.com/nsantiagoblair/k8s-eda/internal/event"
	"github.com/nsantiagoblair/k8s-eda/internal/handler"
	"github.com/nsantiagoblair/k8s-eda/internal/service"
	"github.com/nsantiagoblair/k8s-eda/internal/store"
)

func main() {
	addr    := flag.String("addr",    ":8080",          "address to listen on")
	brokers := flag.String("brokers", "localhost:9092", "comma-separated Kafka broker addresses")
	dbPath  := flag.String("db",      "trades.db",      "path to SQLite database file")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := store.NewSQLiteStore(*dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer s.Close()

	pub := broker.NewKafkaPublisher([]string{*brokers})
	defer pub.Close()

	svc := service.NewTradeService(s, pub)

	mux := http.NewServeMux()
	handler.NewTradeHandler(svc).RegisterRoutes(mux)

	startConsumer(ctx, *brokers, event.TopicTradeFulfilled, "trade-api", consumer.FulfilledHandler(svc))
	startConsumer(ctx, *brokers, event.TopicTradeRejected,  "trade-api", consumer.RejectedHandler(svc))

	srv := &http.Server{Addr: *addr, Handler: mux}
	go func() {
		log.Printf("trade-api listening on %s", *addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down trade-api...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx) //nolint:errcheck
}

func startConsumer(ctx context.Context, brokerAddr, topic, groupID string, h broker.Handler) {
	c := broker.NewKafkaConsumer([]string{brokerAddr}, topic, groupID)
	go func() {
		defer c.Close()
		if err := c.Run(ctx, h); err != nil {
			log.Printf("consumer %s stopped: %v", topic, err)
		}
	}()
}
