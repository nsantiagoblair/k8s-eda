// Package event defines the domain events that flow between services.
package event

import "time"

const (
	TopicTradeSubmitted = "trade-submitted"
	TopicTradeFulfilled = "trade-fulfilled"
	TopicTradeRejected  = "trade-rejected"
)

type TradeSubmitted struct {
	TradeID    string    `json:"tradeId"`
	Asset      string    `json:"asset"`
	Side       string    `json:"side"`
	Quantity   int       `json:"quantity"`
	LimitPrice float64   `json:"limitPrice"`
	OccurredAt time.Time `json:"occurredAt"`
}

type TradeFulfilled struct {
	TradeID     string    `json:"tradeId"`
	FilledPrice float64   `json:"filledPrice"`
	FilledAt    time.Time `json:"filledAt"`
	OccurredAt  time.Time `json:"occurredAt"`
}

type TradeRejected struct {
	TradeID    string    `json:"tradeId"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurredAt"`
}
