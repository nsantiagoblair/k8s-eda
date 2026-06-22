// Provider stub — consumes TradeSubmitted events, evaluates each order, and
// publishes TradeFulfilled or TradeRejected back to Kafka.
//
// In Chapter 4 this was an HTTP server. In Chapter 5 it becomes a pure
// event-driven service: no HTTP port, no direct coupling to the trade API.
// Neither service needs to know the other's address.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nsantiagoblair/k8s-eda/broker"
	"github.com/nsantiagoblair/k8s-eda/event"
)

func main() {
	brokers := flag.String("brokers", "localhost:9092", "comma-separated Kafka broker addresses")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pub := broker.NewKafkaPublisher([]string{*brokers})
	defer pub.Close()

	h := &orderHandler{publisher: pub}

	c := broker.NewKafkaConsumer([]string{*brokers}, event.TopicTradeSubmitted, "provider-stub")
	defer c.Close()

	log.Println("provider stub listening for TradeSubmitted events...")
	if err := c.Run(ctx, h.handle); err != nil {
		log.Printf("consumer stopped: %v", err)
	}
}

type orderHandler struct {
	publisher broker.Publisher
}

func (h *orderHandler) handle(ctx context.Context, msg []byte) error {
	var e event.TradeSubmitted
	if err := json.Unmarshal(msg, &e); err != nil {
		return err
	}

	log.Printf("evaluating order: tradeID=%s asset=%s side=%s qty=%d limit=%.2f",
		e.TradeID, e.Asset, e.Side, e.Quantity, e.LimitPrice)

	return h.evaluate(ctx, e)
}

const minimumPrice = 10.00

func (h *orderHandler) evaluate(ctx context.Context, e event.TradeSubmitted) error {
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
