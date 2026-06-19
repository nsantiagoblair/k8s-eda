package trade

import (
	"fmt"
	"sync"
)

// Store defines the operations for persisting and retrieving trades.
//
// Defining behaviour as an interface means we can swap the implementation
// (in-memory → database) without changing any code that depends on it.
type Store interface {
	Save(t *Trade) error
	FindByID(id string) (*Trade, error)
	FindAll() ([]*Trade, error)
	FindByStatus(status Status) ([]*Trade, error)
}

// ErrNotFound is returned when a trade cannot be located by its ID.
type ErrNotFound struct {
	ID string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("trade not found: %s", e.ID)
}

// InMemoryStore is a thread-safe, in-memory implementation of Store.
// It holds trades in a plain map guarded by a read/write mutex.
type InMemoryStore struct {
	mu     sync.RWMutex
	trades map[string]*Trade
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		trades: make(map[string]*Trade),
	}
}

func (s *InMemoryStore) Save(t *Trade) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.trades[t.ID] = t
	return nil
}

func (s *InMemoryStore) FindByID(id string) (*Trade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.trades[id]
	if !ok {
		return nil, &ErrNotFound{ID: id}
	}
	return t, nil
}

func (s *InMemoryStore) FindAll() ([]*Trade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Trade, 0, len(s.trades))
	for _, t := range s.trades {
		result = append(result, t)
	}
	return result, nil
}

func (s *InMemoryStore) FindByStatus(status Status) ([]*Trade, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Trade
	for _, t := range s.trades {
		if t.Status == status {
			result = append(result, t)
		}
	}
	return result, nil
}
