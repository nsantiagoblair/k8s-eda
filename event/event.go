// Package event defines the domain events that flow between services and the
// Kafka topic names they are published to.
package event

import "time"

// Topic names — centralised here so both publishers and consumers reference
// the same string constants and a typo can't cause a silent mismatch.
const (
	TopicTradeSubmitted = "trade-submitted"
	TopicTradeFulfilled = "trade-fulfilled"
	TopicTradeRejected  = "trade-rejected"
)

// TradeSubmitted is published by the trade API when a trade is ready for
// execution. The provider stub consumes this event and processes the order.
type TradeSubmitted struct {
	TradeID    string    `json:"tradeId"`
	Asset      string    `json:"asset"`
	Side       string    `json:"side"`
	Quantity   int       `json:"quantity"`
	LimitPrice float64   `json:"limitPrice"`
	OccurredAt time.Time `json:"occurredAt"`
}

// TradeFulfilled is published by the provider stub when an order has been
// successfully executed. The trade API consumes this event and transitions the
// trade to FULFILLED.
type TradeFulfilled struct {
	TradeID     string    `json:"tradeId"`
	FilledPrice float64   `json:"filledPrice"`
	FilledAt    time.Time `json:"filledAt"`
	OccurredAt  time.Time `json:"occurredAt"`
}

// TradeRejected is published by the provider stub when an order cannot be
// executed (e.g. price below threshold). The trade API consumes this event and
// transitions the trade to REJECTED.
type TradeRejected struct {
	TradeID    string    `json:"tradeId"`
	Reason     string    `json:"reason"`
	OccurredAt time.Time `json:"occurredAt"`
}
