package consumer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nsantiagoblair/k8s-eda/consumer"
	"github.com/nsantiagoblair/k8s-eda/event"
	"github.com/nsantiagoblair/k8s-eda/store"
	"github.com/nsantiagoblair/k8s-eda/trade"
)

// newSubmittedTrade creates a trade that has already been transitioned to
// SUBMITTED — the state the consumer expects to receive events for.
func newSubmittedTrade(t *testing.T, s trade.Store) *trade.Trade {
	t.Helper()
	tr, err := trade.New("AAPL", trade.Buy, 10, 150.00)
	if err != nil {
		t.Fatalf("trade.New: %v", err)
	}
	if err := tr.Transition(trade.Submitted); err != nil {
		t.Fatalf("Transition: %v", err)
	}
	if err := s.Save(tr); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return tr
}

func newTestStore(t *testing.T) trade.Store {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestHandleFulfilled(t *testing.T) {
	s := newTestStore(t)
	tr := newSubmittedTrade(t, s)

	tc := consumer.NewTradeConsumer(s)
	evt := event.TradeFulfilled{
		TradeID:     tr.ID,
		FilledPrice: 150.00,
		FilledAt:    time.Now(),
		OccurredAt:  time.Now(),
	}
	data, _ := json.Marshal(evt)

	if err := tc.HandleFulfilled(context.Background(), data); err != nil {
		t.Fatalf("HandleFulfilled: %v", err)
	}

	got, err := s.FindByID(tr.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Status != trade.Fulfilled {
		t.Errorf("Status = %s; want %s", got.Status, trade.Fulfilled)
	}
}

func TestHandleRejected(t *testing.T) {
	s := newTestStore(t)
	tr := newSubmittedTrade(t, s)

	tc := consumer.NewTradeConsumer(s)
	evt := event.TradeRejected{
		TradeID:    tr.ID,
		Reason:     "below minimum price",
		OccurredAt: time.Now(),
	}
	data, _ := json.Marshal(evt)

	if err := tc.HandleRejected(context.Background(), data); err != nil {
		t.Fatalf("HandleRejected: %v", err)
	}

	got, err := s.FindByID(tr.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Status != trade.Rejected {
		t.Errorf("Status = %s; want %s", got.Status, trade.Rejected)
	}
}

func TestHandleFulfilled_UnknownTrade(t *testing.T) {
	s := newTestStore(t)
	tc := consumer.NewTradeConsumer(s)

	evt := event.TradeFulfilled{
		TradeID:    "no-such-id",
		OccurredAt: time.Now(),
	}
	data, _ := json.Marshal(evt)

	if err := tc.HandleFulfilled(context.Background(), data); err == nil {
		t.Error("expected an error for unknown trade ID, got nil")
	}
}

func TestHandleFulfilled_InvalidJSON(t *testing.T) {
	s := newTestStore(t)
	tc := consumer.NewTradeConsumer(s)

	if err := tc.HandleFulfilled(context.Background(), []byte("not json")); err == nil {
		t.Error("expected unmarshal error, got nil")
	}
}
