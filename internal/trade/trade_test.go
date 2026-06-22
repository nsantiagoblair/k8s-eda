package trade_test

import (
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		asset      string
		side       trade.Side
		quantity   int
		limitPrice float64
		wantErr    bool
	}{
		{name: "valid buy",         asset: "AAPL", side: trade.Buy,  quantity: 10, limitPrice: 195.00, wantErr: false},
		{name: "valid sell",        asset: "TSLA", side: trade.Sell, quantity: 5,  limitPrice: 250.00, wantErr: false},
		{name: "empty asset",       asset: "",     side: trade.Buy,  quantity: 10, limitPrice: 195.00, wantErr: true},
		{name: "zero quantity",     asset: "AAPL", side: trade.Buy,  quantity: 0,  limitPrice: 195.00, wantErr: true},
		{name: "negative quantity", asset: "AAPL", side: trade.Buy,  quantity: -1, limitPrice: 195.00, wantErr: true},
		{name: "zero price",        asset: "AAPL", side: trade.Buy,  quantity: 10, limitPrice: 0,      wantErr: true},
		{name: "negative price",    asset: "AAPL", side: trade.Buy,  quantity: 10, limitPrice: -1,     wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := trade.New(tt.asset, tt.side, tt.quantity, tt.limitPrice)
			if (err != nil) != tt.wantErr {
				t.Fatalf("New() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.ID == "" { t.Error("expected non-empty ID") }
			if got.Status != trade.Pending { t.Errorf("expected PENDING, got %s", got.Status) }
			if got.CreatedAt.IsZero() { t.Error("expected CreatedAt to be set") }
		})
	}
}

func TestTransition(t *testing.T) {
	tests := []struct {
		name    string
		from    trade.Status
		to      trade.Status
		wantErr bool
	}{
		{name: "pending → submitted",        from: trade.Pending,   to: trade.Submitted, wantErr: false},
		{name: "submitted → fulfilled",      from: trade.Submitted, to: trade.Fulfilled, wantErr: false},
		{name: "submitted → rejected",       from: trade.Submitted, to: trade.Rejected,  wantErr: false},
		{name: "pending → fulfilled (skip)", from: trade.Pending,   to: trade.Fulfilled, wantErr: true},
		{name: "fulfilled → submitted",      from: trade.Fulfilled, to: trade.Submitted, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
			tr.Status = tt.from

			err := tr.Transition(tt.to)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Transition() error = %v, wantErr %v", err, tt.wantErr)
			}
			// Invalid transitions should return a typed error.
			if tt.wantErr {
				var inv *trade.ErrInvalidTransition
				if !errors.As(err, &inv) {
					t.Errorf("expected ErrInvalidTransition, got %T: %v", err, err)
				}
			}
		})
	}
}

func TestTransition_UpdatesTimestamp(t *testing.T) {
	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	before := tr.UpdatedAt
	tr.Transition(trade.Submitted) //nolint:errcheck
	if !tr.UpdatedAt.After(before) {
		t.Error("expected UpdatedAt to advance after transition")
	}
}
