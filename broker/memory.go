package broker

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// InMemoryPublisher is a thread-safe Publisher that stores messages in memory.
// It is used in tests to capture published events without needing a running
// Kafka broker.
type InMemoryPublisher struct {
	mu       sync.RWMutex
	messages map[string][][]byte
}

func NewInMemoryPublisher() *InMemoryPublisher {
	return &InMemoryPublisher{
		messages: make(map[string][][]byte),
	}
}

func (p *InMemoryPublisher) Publish(_ context.Context, topic string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages[topic] = append(p.messages[topic], data)
	return nil
}

func (p *InMemoryPublisher) Close() error { return nil }

// Messages returns the raw JSON payloads published to topic.
func (p *InMemoryPublisher) Messages(topic string) [][]byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.messages[topic]
}

// Count returns the number of messages published to topic.
func (p *InMemoryPublisher) Count(topic string) int {
	return len(p.Messages(topic))
}
