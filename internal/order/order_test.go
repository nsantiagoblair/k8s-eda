package order_test

import (
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/order"
)

func TestNew(t *testing.T) {
	o := order.New("trade-1", "AAPL", "BUY", 10, 195.00)
	if o.ID == ""             { t.Error("expected non-empty ID") }
	if o.TradeID != "trade-1" { t.Errorf("TradeID = %s; want trade-1", o.TradeID) }
	if o.Status != order.Pending { t.Errorf("Status = %s; want PENDING", o.Status) }
	if o.CreatedAt.IsZero()   { t.Error("expected CreatedAt to be set") }
}

func TestTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    order.Status
		to      order.Status
		wantErr bool
	}{
		{name: "pending → filled",      from: order.Pending,   to: order.Filled,     wantErr: false},
		{name: "pending → cancelled",   from: order.Pending,   to: order.Cancelled,  wantErr: false},
		{name: "filled → cancelled",    from: order.Filled,    to: order.Cancelled,  wantErr: true},
		{name: "cancelled → filled",    from: order.Cancelled, to: order.Filled,     wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := order.New("t", "AAPL", "BUY", 1, 100)
			o.Status = tt.from
			err := o.Transition(tt.to)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Transition() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				var inv *order.ErrInvalidTransition
				if !errors.As(err, &inv) {
					t.Errorf("expected ErrInvalidTransition, got %T", err)
				}
			}
		})
	}
}
