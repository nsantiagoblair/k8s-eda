package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
	"github.com/nsantiagoblair/k8s-eda/internal/event"
	"github.com/nsantiagoblair/k8s-eda/internal/infra/kafka"
)

func main() {
	brokers := flag.String("brokers", "localhost:9092", "comma-separated Kafka broker addresses")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pub := kafka.NewPublisher([]string{*brokers})
	defer pub.Close()

	h := &orderHandler{publisher: pub}

	c := kafka.NewConsumer([]string{*brokers}, event.TopicTradeSubmitted, "provider-stub")
	defer c.Close()

	// Use the generic HandlerFunc to handle unmarshalling automatically.
	log.Println("provider stub listening for TradeSubmitted events...")
	if err := c.Run(ctx, broker.HandlerFunc[event.TradeSubmitted](h.evaluate)); err != nil {
		log.Printf("consumer stopped: %v", err)
	}
}

type orderHandler struct {
	publisher broker.Publisher
}

const minimumPrice = 10.00

func (h *orderHandler) evaluate(ctx context.Context, e event.TradeSubmitted) error {
	log.Printf("evaluating order: tradeID=%s asset=%s side=%s qty=%d limit=%.2f",
		e.TradeID, e.Asset, e.Side, e.Quantity, e.LimitPrice)

	if e.LimitPrice < minimumPrice {
		return h.publisher.Publish(ctx, event.TopicTradeRejected, event.TradeRejected{
			TradeID:    e.TradeID,
			Reason:     "limit price below minimum accepted threshold",
			OccurredAt: time.Now(),
		})
	}

	return h.publisher.Publish(ctx, event.TopicTradeFulfilled, event.TradeFulfilled{
		TradeID:     e.TradeID,
		FilledPrice: e.LimitPrice,
		FilledAt:    time.Now(),
		OccurredAt:  time.Now(),
	})
}

