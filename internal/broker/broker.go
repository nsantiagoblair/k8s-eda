// Package broker defines the interfaces for publishing and consuming messages,
// and provides Kafka and in-memory implementations.
package broker

import (
	"context"
	"encoding/json"
	"fmt"
)

// Publisher publishes messages to a named topic.
type Publisher interface {
	Publish(ctx context.Context, topic string, v any) error
	Close() error
}

// Handler is the function signature for processing a consumed message.
// Returning an error logs the failure; the offset is still committed
// (at-least-once delivery). Chapter 9 adds a dead-letter queue for
// persistent failures.
type Handler func(ctx context.Context, msg []byte) error

// Consumer reads messages from a single topic and calls a Handler for each one.
type Consumer interface {
	// Run blocks until ctx is cancelled, calling handler for every message.
	Run(ctx context.Context, handler Handler) error
	Close() error
}

// HandlerFunc wraps a strongly-typed function as a broker.Handler, automatically
// handling JSON unmarshalling. This eliminates the boilerplate json.Unmarshal
// call that would otherwise appear in every consumer method.
//
// Example:
//
//	broker.HandlerFunc[event.TradeFulfilled](func(ctx context.Context, e event.TradeFulfilled) error {
//	    return svc.Fulfill(e.TradeID)
//	})
//
// HandlerFunc is a generic function (not a method), introduced in Go 1.18.
// The type parameter T is inferred from the argument function's signature.
func HandlerFunc[T any](fn func(context.Context, T) error) Handler {
	return func(ctx context.Context, msg []byte) error {
		var v T
		if err := json.Unmarshal(msg, &v); err != nil {
			return fmt.Errorf("unmarshal %T: %w", v, err)
		}
		return fn(ctx, v)
	}
}
