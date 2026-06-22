// Package consumer builds broker.Handler functions for trade events.
// Each constructor takes the service it delegates to and returns a Handler
// ready to pass to broker.KafkaConsumer.Run or broker.NewKafkaConsumer.
package consumer

import (
	"context"

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
	"github.com/nsantiagoblair/k8s-eda/internal/event"
)

// tradeService is the subset of service.TradeService the consumers need.
type tradeService interface {
	Fulfill(tradeID string) error
	Reject(tradeID string) error
}

// FulfilledHandler returns a broker.Handler that transitions a trade to FULFILLED
// when a TradeFulfilled event is received.
//
// broker.HandlerFunc[T] handles JSON unmarshalling, so this function only needs
// to express the business logic — a single line.
func FulfilledHandler(svc tradeService) broker.Handler {
	return broker.HandlerFunc[event.TradeFulfilled](func(_ context.Context, e event.TradeFulfilled) error {
		return svc.Fulfill(e.TradeID)
	})
}

// RejectedHandler returns a broker.Handler that transitions a trade to REJECTED
// when a TradeRejected event is received.
func RejectedHandler(svc tradeService) broker.Handler {
	return broker.HandlerFunc[event.TradeRejected](func(_ context.Context, e event.TradeRejected) error {
		return svc.Reject(e.TradeID)
	})
}
