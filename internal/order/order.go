// Package order defines the Order domain model.
// An Order represents a trade instruction that has been sent to the provider
// for execution. It is created from a TradeSubmitted event and tracks the
// provider's response (Filled or Cancelled).
//
// Order is a separate entity from Trade — a trade is an instruction from a
// user; an order is a record of what was sent to the market on their behalf.
// In a real system they might diverge (partial fills, order splitting, etc.).
package order

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Status represents where an order is in its execution lifecycle.
//
//	PENDING → FILLED
//	        ↘ CANCELLED
type Status int

const (
	Pending   Status = iota
	Filled           // provider confirmed execution
	Cancelled        // provider rejected or could not execute
)

func (s Status) String() string {
	switch s {
	case Pending:
		return "PENDING"
	case Filled:
		return "FILLED"
	case Cancelled:
		return "CANCELLED"
	default:
		return "UNKNOWN"
	}
}

func ParseStatus(s string) (Status, error) {
	switch s {
	case "PENDING":
		return Pending, nil
	case "FILLED":
		return Filled, nil
	case "CANCELLED":
		return Cancelled, nil
	default:
		return 0, fmt.Errorf("unknown order status %q", s)
	}
}

func (s Status) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }

// Order records a single execution attempt sent to the provider.
type Order struct {
	ID        string    `json:"id"`
	TradeID   string    `json:"tradeId"`  // the trade that triggered this order
	Asset     string    `json:"asset"`
	Side      string    `json:"side"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// New creates a new Order in Pending status for the given trade details.
func New(tradeID, asset, side string, quantity int, price float64) *Order {
	now := time.Now()
	return &Order{
		ID:        uuid.NewString(),
		TradeID:   tradeID,
		Asset:     asset,
		Side:      side,
		Quantity:  quantity,
		Price:     price,
		Status:    Pending,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ErrInvalidTransition is returned when a status change is not permitted.
type ErrInvalidTransition struct {
	From Status
	To   Status
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("cannot transition order from %s to %s", e.From, e.To)
}

// Transition moves the order to a new status.
func (o *Order) Transition(next Status) error {
	for _, allowed := range validTransitions[o.Status] {
		if allowed == next {
			o.Status = next
			o.UpdatedAt = time.Now()
			return nil
		}
	}
	return &ErrInvalidTransition{From: o.Status, To: next}
}

var validTransitions = map[Status][]Status{
	Pending:   {Filled, Cancelled},
	Filled:    {},
	Cancelled: {},
}
