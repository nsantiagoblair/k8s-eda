// Package trade defines the core domain model for the share trading system.
package trade

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Side represents whether a trade is a buy or a sell.
type Side int

const (
	Buy Side = iota
	Sell
)

func (s Side) String() string {
	switch s {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	default:
		return "UNKNOWN"
	}
}

// Status represents where a trade is in its lifecycle.
//
//	PENDING → SUBMITTED → FULFILLED
//	                    ↘ REJECTED
type Status int

const (
	Pending Status = iota
	Submitted
	Fulfilled
	Rejected
)

func (s Status) String() string {
	switch s {
	case Pending:
		return "PENDING"
	case Submitted:
		return "SUBMITTED"
	case Fulfilled:
		return "FULFILLED"
	case Rejected:
		return "REJECTED"
	default:
		return "UNKNOWN"
	}
}

// Trade is an instruction to buy or sell a quantity of an asset.
type Trade struct {
	ID         string
	Asset      string  // ticker symbol, e.g. "AAPL"
	Side       Side
	Quantity   int
	LimitPrice float64 // maximum price willing to pay (BUY) or minimum to accept (SELL)
	Status     Status
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// New creates a new Trade in Pending status.
func New(asset string, side Side, quantity int, limitPrice float64) (*Trade, error) {
	if asset == "" {
		return nil, fmt.Errorf("asset must not be empty")
	}
	if quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive, got %d", quantity)
	}
	if limitPrice <= 0 {
		return nil, fmt.Errorf("limit price must be positive, got %.2f", limitPrice)
	}

	now := time.Now()
	return &Trade{
		ID:         uuid.NewString(),
		Asset:      asset,
		Side:       side,
		Quantity:   quantity,
		LimitPrice: limitPrice,
		Status:     Pending,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// Transition moves the trade to a new status, returning an error if the
// transition is not valid for the current state.
func (t *Trade) Transition(next Status) error {
	valid := validTransitions[t.Status]
	for _, allowed := range valid {
		if allowed == next {
			t.Status = next
			t.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("cannot transition trade from %s to %s", t.Status, next)
}

// validTransitions maps each status to the statuses it may move to.
var validTransitions = map[Status][]Status{
	Pending:   {Submitted},
	Submitted: {Fulfilled, Rejected},
	Fulfilled: {},
	Rejected:  {},
}

func (t *Trade) String() string {
	return fmt.Sprintf(
		"Trade{ID: %s, Asset: %s, Side: %s, Qty: %d, Limit: %.2f, Status: %s}",
		t.ID[:8], t.Asset, t.Side, t.Quantity, t.LimitPrice, t.Status,
	)
}
