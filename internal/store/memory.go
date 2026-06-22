package store

import "sync"

// InMemoryStore[T] is a thread-safe, generic in-memory implementation of Store[T].
//
// The keyOf function extracts the string ID from an entity so the store knows
// which map slot to use. For a *trade.Trade you'd pass:
//
//	store.NewInMemoryStore(func(t *trade.Trade) string { return t.ID })
//
// This is a common Go generics pattern: instead of requiring T to implement an
// interface (e.g. Identifiable), we accept the extraction logic as a plain
// function. The caller knows their type; the store stays ignorant of it.
type InMemoryStore[T any] struct {
	mu    sync.RWMutex
	items map[string]T
	keyOf func(T) string
}

func NewInMemoryStore[T any](keyOf func(T) string) *InMemoryStore[T] {
	return &InMemoryStore[T]{
		items: make(map[string]T),
		keyOf: keyOf,
	}
}

func (s *InMemoryStore[T]) Save(t T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[s.keyOf(t)] = t
	return nil
}

func (s *InMemoryStore[T]) FindByID(id string) (T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.items[id]
	if !ok {
		// var zero T gives the zero value for T — nil for pointer types.
		var zero T
		return zero, &ErrNotFound{ID: id}
	}
	return t, nil
}

func (s *InMemoryStore[T]) FindAll() ([]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]T, 0, len(s.items))
	for _, v := range s.items {
		out = append(out, v)
	}
	return out, nil
}
