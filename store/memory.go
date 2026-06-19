package store

import (
	"sync"

	"github.com/nsantiagoblair/k8s-eda/trade"
)

// InMemoryStore is a thread-safe, in-memory implementation of trade.Store.
// It is useful for tests and for running the server without a database.
type InMemoryStore struct {
	mu     sync.RWMutex
	trades map[string]*trade.Trade
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		trades: make(map[string]*trade.Trade),
	}
}

func (s *InMemoryStore) Save(t *trade.Trade) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trades[t.ID] = t
	return nil
}

func (s *InMemoryStore) FindByID(id string) (*trade.Trade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.trades[id]
	if !ok {
		return nil, &trade.ErrNotFound{ID: id}
	}
	return t, nil
}

func (s *InMemoryStore) FindAll() ([]*trade.Trade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*trade.Trade, 0, len(s.trades))
	for _, t := range s.trades {
		result = append(result, t)
	}
	return result, nil
}

func (s *InMemoryStore) FindByStatus(status trade.Status) ([]*trade.Trade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*trade.Trade
	for _, t := range s.trades {
		if t.Status == status {
			result = append(result, t)
		}
	}
	if result == nil {
		result = []*trade.Trade{}
	}
	return result, nil
}
