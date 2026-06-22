// Package service contains the application service layer.
// A service orchestrates domain logic and coordinates between the store and
// the broker. It knows about domain types but nothing about HTTP or Kafka
// message formats — those concerns belong to the handler and consumer packages.
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
	"github.com/nsantiagoblair/k8s-eda/internal/event"
	"github.com/nsantiagoblair/k8s-eda/internal/store"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

// TradeService orchestrates trade operations.
// It sits between the transport layer (HTTP handlers, Kafka consumers) and the
// infrastructure layer (stores, brokers), keeping each layer ignorant of the others.
type TradeService struct {
	store store.Store[*trade.Trade]
	pub   broker.Publisher
}

func NewTradeService(s store.Store[*trade.Trade], pub broker.Publisher) *TradeService {
	return &TradeService{store: s, pub: pub}
}

// Create validates and persists a new trade in PENDING status.
func (s *TradeService) Create(asset string, side trade.Side, qty int, price float64) (*trade.Trade, error) {
	t, err := trade.New(asset, side, qty, price)
	if err != nil {
		return nil, err
	}
	return t, s.store.Save(t)
}

// Submit transitions a trade to SUBMITTED and publishes a TradeSubmitted event.
// The trade's final status (FULFILLED or REJECTED) arrives asynchronously.
func (s *TradeService) Submit(ctx context.Context, id string) (*trade.Trade, error) {
	t, err := s.store.FindByID(id)
	if err != nil {
		return nil, err
	}

	if err := t.Transition(trade.Submitted); err != nil {
		return nil, err
	}
	if err := s.store.Save(t); err != nil {
		return nil, fmt.Errorf("save after submit: %w", err)
	}

	evt := event.TradeSubmitted{
		TradeID:    t.ID,
		Asset:      t.Asset,
		Side:       t.Side.String(),
		Quantity:   t.Quantity,
		LimitPrice: t.LimitPrice,
		OccurredAt: time.Now(),
	}
	if err := s.pub.Publish(ctx, event.TopicTradeSubmitted, evt); err != nil {
		return nil, fmt.Errorf("publish event: %w", err)
	}

	return t, nil
}

// GetByID retrieves a trade by its ID.
func (s *TradeService) GetByID(id string) (*trade.Trade, error) {
	return s.store.FindByID(id)
}

// List returns all trades.
func (s *TradeService) List() ([]*trade.Trade, error) {
	return s.store.FindAll()
}

// Fulfill transitions a trade from SUBMITTED to FULFILLED.
// Called by the event consumer when a TradeFulfilled event arrives.
func (s *TradeService) Fulfill(tradeID string) error {
	return s.applyTransition(tradeID, trade.Fulfilled)
}

// Reject transitions a trade from SUBMITTED to REJECTED.
// Called by the event consumer when a TradeRejected event arrives.
func (s *TradeService) Reject(tradeID string) error {
	return s.applyTransition(tradeID, trade.Rejected)
}

func (s *TradeService) applyTransition(tradeID string, status trade.Status) error {
	t, err := s.store.FindByID(tradeID)
	if err != nil {
		return fmt.Errorf("find trade %s: %w", tradeID, err)
	}
	if err := t.Transition(status); err != nil {
		return fmt.Errorf("transition trade %s: %w", tradeID, err)
	}
	return s.store.Save(t)
}
