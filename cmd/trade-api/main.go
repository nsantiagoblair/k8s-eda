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
	"github.com/nsantiagoblair/k8s-eda/internal/infra/kafka"
	"github.com/nsantiagoblair/k8s-eda/internal/infra/memory"
	"github.com/nsantiagoblair/k8s-eda/internal/infra/sqlite"
	"github.com/nsantiagoblair/k8s-eda/internal/order"
	"github.com/nsantiagoblair/k8s-eda/internal/service"
)

func main() {
	addr    := flag.String("addr",    ":8080",          "address to listen on")
	brokers := flag.String("brokers", "localhost:9092", "Kafka broker address")
	dbPath  := flag.String("db",      "trades.db",      "SQLite database path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokerList := []string{*brokers}

	// --- stores -----------------------------------------------------------------
	// Trades are persisted to SQLite; orders use an in-memory store for now.
	// Chapter 9 replaces both with Postgres.
	tradeStore, err := sqlite.NewStore(*dbPath)
	if err != nil {
		log.Fatalf("open trade store: %v", err)
	}
	defer tradeStore.Close()

	orderStore := memory.NewStore(func(o *order.Order) string { return o.ID })

	// --- broker -----------------------------------------------------------------
	pub := kafka.NewPublisher(brokerList)
	defer pub.Close()

	// --- services ---------------------------------------------------------------
	tradeSvc := service.NewTradeService(tradeStore, pub)
	orderSvc := service.NewOrderService(orderStore)

	// --- HTTP -------------------------------------------------------------------
	mux := http.NewServeMux()
	handler.NewTradeHandler(tradeSvc).RegisterRoutes(mux)
	handler.NewOrderHandler(orderSvc).RegisterRoutes(mux)

	// --- Kafka consumers --------------------------------------------------------
	// Trade status consumers (consumer group: trade-api-trades)
	startConsumer(ctx, brokerList, event.TopicTradeFulfilled, "trade-api-trades", consumer.FulfilledHandler(tradeSvc))
	startConsumer(ctx, brokerList, event.TopicTradeRejected,  "trade-api-trades", consumer.RejectedHandler(tradeSvc))

	// Order lifecycle consumers (consumer group: trade-api-orders)
	// Each topic can have multiple independent consumer groups reading from it.
	startConsumer(ctx, brokerList, event.TopicTradeSubmitted,  "trade-api-orders", consumer.OrderCreatedHandler(orderSvc))
	startConsumer(ctx, brokerList, event.TopicTradeFulfilled,  "trade-api-orders", consumer.OrderFilledHandler(orderSvc))
	startConsumer(ctx, brokerList, event.TopicTradeRejected,   "trade-api-orders", consumer.OrderCancelledHandler(orderSvc))

	// --- HTTP server ------------------------------------------------------------
	srv := &http.Server{Addr: *addr, Handler: mux}
	go func() {
		log.Printf("trade-api listening on %s", *addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx) //nolint:errcheck
}

func startConsumer(ctx context.Context, brokers []string, topic, groupID string, h broker.Handler) {
	c := kafka.NewConsumer(brokers, topic, groupID)
	go func() {
		defer c.Close()
		if err := c.Run(ctx, h); err != nil {
			log.Printf("consumer %s/%s stopped: %v", topic, groupID, err)
		}
	}()
}

