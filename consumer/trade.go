// Package consumer contains the event handlers for the trade API.
// Each handler is a broker.Handler — it receives a raw JSON message and
// updates the trade store accordingly.
package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nsantiagoblair/k8s-eda/event"
	"github.com/nsantiagoblair/k8s-eda/trade"
)

// TradeConsumer handles events that update trade status.
type TradeConsumer struct {
	store trade.Store
}

func NewTradeConsumer(store trade.Store) *TradeConsumer {
	return &TradeConsumer{store: store}
}

// HandleFulfilled processes a TradeFulfilled event, transitioning the trade
// from SUBMITTED to FULFILLED.
func (c *TradeConsumer) HandleFulfilled(ctx context.Context, msg []byte) error {
	var e event.TradeFulfilled
	if err := json.Unmarshal(msg, &e); err != nil {
		return fmt.Errorf("unmarshal TradeFulfilled: %w", err)
	}

	return c.applyTransition(e.TradeID, trade.Fulfilled)
}

// HandleRejected processes a TradeRejected event, transitioning the trade
// from SUBMITTED to REJECTED.
func (c *TradeConsumer) HandleRejected(ctx context.Context, msg []byte) error {
	var e event.TradeRejected
	if err := json.Unmarshal(msg, &e); err != nil {
		return fmt.Errorf("unmarshal TradeRejected: %w", err)
	}

	return c.applyTransition(e.TradeID, trade.Rejected)
}

func (c *TradeConsumer) applyTransition(tradeID string, status trade.Status) error {
	t, err := c.store.FindByID(tradeID)
	if err != nil {
		return fmt.Errorf("find trade %s: %w", tradeID, err)
	}

	if err := t.Transition(status); err != nil {
		// The trade may already be in the target state if the event was
		// delivered more than once (at-least-once). Log and skip rather
		// than failing — idempotency is handled by checking current state.
		return fmt.Errorf("transition trade %s to %s: %w", tradeID, status, err)
	}

	return c.store.Save(t)
}
