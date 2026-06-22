// Package trade defines the core domain model for the share trading system.
// It contains only pure domain logic — no HTTP, no database, no broker imports.
package trade

import (
	"encoding/json"
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

// ParseSide converts a string ("BUY" or "SELL") to a Side value.
func ParseSide(s string) (Side, error) {
	switch s {
	case "BUY":
		return Buy, nil
	case "SELL":
		return Sell, nil
	default:
		return 0, fmt.Errorf("unknown side %q, must be BUY or SELL", s)
	}
}

func (s Side) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *Side) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	parsed, err := ParseSide(str)
	if err != nil {
		return err
	}
	*s = parsed
	return nil
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

// ParseStatus converts a status string to a Status value.
func ParseStatus(s string) (Status, error) {
	switch s {
	case "PENDING":
		return Pending, nil
	case "SUBMITTED":
		return Submitted, nil
	case "FULFILLED":
		return Fulfilled, nil
	case "REJECTED":
		return Rejected, nil
	default:
		return 0, fmt.Errorf("unknown status %q", s)
	}
}

func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// Trade is an instruction to buy or sell a quantity of an asset.
type Trade struct {
	ID         string    `json:"id"`
	Asset      string    `json:"asset"`
	Side       Side      `json:"side"`
	Quantity   int       `json:"quantity"`
	LimitPrice float64   `json:"limitPrice"`
	Status     Status    `json:"status"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
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

// ErrInvalidTransition is returned when a status change is not permitted.
// Using a typed error lets callers (e.g. the HTTP handler) distinguish a
// domain rule violation from an unexpected error and map it to 409 Conflict.
type ErrInvalidTransition struct {
	From Status
	To   Status
}

func (e *ErrInvalidTransition) Error() string {
	return fmt.Sprintf("cannot transition trade from %s to %s", e.From, e.To)
}

// Transition moves the trade to a new status, returning ErrInvalidTransition
// if the move is not allowed from the current state.
func (t *Trade) Transition(next Status) error {
	for _, allowed := range validTransitions[t.Status] {
		if allowed == next {
			t.Status = next
			t.UpdatedAt = time.Now()
			return nil
		}
	}
	return &ErrInvalidTransition{From: t.Status, To: next}
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
