package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Publisher is a thread-safe in-memory broker.Publisher used in tests to
// capture published events without a running Kafka broker.
type Publisher struct {
	mu       sync.RWMutex
	messages map[string][][]byte
}

func NewPublisher() *Publisher {
	return &Publisher{messages: make(map[string][][]byte)}
}

func (p *Publisher) Publish(_ context.Context, topic string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.messages[topic] = append(p.messages[topic], data)
	return nil
}

func (p *Publisher) Close() error { return nil }

func (p *Publisher) Messages(topic string) [][]byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.messages[topic]
}

func (p *Publisher) Count(topic string) int { return len(p.Messages(topic)) }
