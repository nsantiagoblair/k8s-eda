// Package provider defines the interface and types for communicating with an
// external trade execution provider.
package provider

import (
	"context"
	"time"
)

// Provider is the port through which the trade API submits orders for execution.
// The concrete implementation (Client) calls a real HTTP service; in tests a
// mock can be substituted without changing any handler code.
type Provider interface {
	Submit(ctx context.Context, req SubmitRequest) (*SubmitResponse, error)
}

// SubmitRequest is sent to the provider when a trade is ready for execution.
type SubmitRequest struct {
	TradeID    string  `json:"tradeId"`
	Asset      string  `json:"asset"`
	Side       string  `json:"side"` // "BUY" or "SELL"
	Quantity   int     `json:"quantity"`
	LimitPrice float64 `json:"limitPrice"`
}

// SubmitResponse is returned by the provider once it has processed the order.
type SubmitResponse struct {
	TradeID     string    `json:"tradeId"`
	Status      string    `json:"status"`                // "FULFILLED" or "REJECTED"
	FilledPrice float64   `json:"filledPrice,omitempty"` // set when FULFILLED
	FilledAt    time.Time `json:"filledAt,omitempty"`    // set when FULFILLED
	Reason      string    `json:"reason,omitempty"`      // set when REJECTED
}
