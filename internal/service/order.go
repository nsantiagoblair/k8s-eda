package service

import (
	"fmt"

	"github.com/nsantiagoblair/k8s-eda/internal/event"
	"github.com/nsantiagoblair/k8s-eda/internal/order"
	"github.com/nsantiagoblair/k8s-eda/internal/store"
)

// OrderService manages the lifecycle of orders created from trade events.
// It uses the same store.Store[T] interface as TradeService — no new
// infrastructure code was needed to support a second entity.
type OrderService struct {
	store store.Store[*order.Order]
}

func NewOrderService(s store.Store[*order.Order]) *OrderService {
	return &OrderService{store: s}
}

// CreateFromTrade creates a new PENDING order from a TradeSubmitted event.
// Called by the consumer when trade-api publishes a TradeSubmitted event.
func (s *OrderService) CreateFromTrade(e event.TradeSubmitted) error {
	o := order.New(e.TradeID, e.Asset, e.Side, e.Quantity, e.LimitPrice)
	return s.store.Save(o)
}

// FillByTradeID transitions the order for a given trade to FILLED.
// Called when a TradeFulfilled event arrives from the provider.
func (s *OrderService) FillByTradeID(tradeID string) error {
	return s.applyTransition(tradeID, order.Filled)
}

// CancelByTradeID transitions the order for a given trade to CANCELLED.
// Called when a TradeRejected event arrives from the provider.
func (s *OrderService) CancelByTradeID(tradeID string) error {
	return s.applyTransition(tradeID, order.Cancelled)
}

// GetByID retrieves a single order by its own ID.
func (s *OrderService) GetByID(id string) (*order.Order, error) {
	return s.store.FindByID(id)
}

// List returns all orders.
func (s *OrderService) List() ([]*order.Order, error) {
	return s.store.FindAll()
}

func (s *OrderService) applyTransition(tradeID string, status order.Status) error {
	o, err := s.findByTradeID(tradeID)
	if err != nil {
		return fmt.Errorf("find order for trade %s: %w", tradeID, err)
	}
	if err := o.Transition(status); err != nil {
		return err
	}
	return s.store.Save(o)
}

// findByTradeID scans all orders to find one matching a trade ID.
// This is O(n) — acceptable for the in-memory store. Chapter 9 replaces
// this with a SQL index when we switch to Postgres.
func (s *OrderService) findByTradeID(tradeID string) (*order.Order, error) {
	all, err := s.store.FindAll()
	if err != nil {
		return nil, err
	}
	for _, o := range all {
		if o.TradeID == tradeID {
			return o, nil
		}
	}
	return nil, &store.ErrNotFound{ID: tradeID}
}
