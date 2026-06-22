// Package broker defines the interfaces for publishing and consuming messages,
// and provides Kafka and in-memory implementations.
package broker

import "context"

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
