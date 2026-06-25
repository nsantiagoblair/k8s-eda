package consumer

import (
	"context"

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
	"github.com/nsantiagoblair/k8s-eda/internal/event"
)

// orderService is the subset of service.OrderService the consumers need.
type orderService interface {
	CreateFromTrade(e event.TradeSubmitted) error
	FillByTradeID(tradeID string) error
	CancelByTradeID(tradeID string) error
}

// OrderCreatedHandler returns a broker.Handler that creates a new PENDING order
// whenever a TradeSubmitted event arrives.
func OrderCreatedHandler(svc orderService) broker.Handler {
	return broker.HandlerFunc[event.TradeSubmitted](func(_ context.Context, e event.TradeSubmitted) error {
		return svc.CreateFromTrade(e)
	})
}

// OrderFilledHandler returns a broker.Handler that transitions an order to FILLED
// when a TradeFulfilled event arrives.
func OrderFilledHandler(svc orderService) broker.Handler {
	return broker.HandlerFunc[event.TradeFulfilled](func(_ context.Context, e event.TradeFulfilled) error {
		return svc.FillByTradeID(e.TradeID)
	})
}

// OrderCancelledHandler returns a broker.Handler that transitions an order to CANCELLED
// when a TradeRejected event arrives.
func OrderCancelledHandler(svc orderService) broker.Handler {
	return broker.HandlerFunc[event.TradeRejected](func(_ context.Context, e event.TradeRejected) error {
		return svc.CancelByTradeID(e.TradeID)
	})
}
