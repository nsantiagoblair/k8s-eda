// Package memory provides in-memory implementations of store.Store[T] and
// broker.Publisher. These are used in tests and as a default before a
// persistent store (e.g. sqlite, postgres) is wired up.
package memory

import (
	"sync"

	"github.com/nsantiagoblair/k8s-eda/internal/store"
)

// Store[T] is a thread-safe generic in-memory implementation of store.Store[T].
// The keyOf function extracts the string ID from any entity T so the store
// stays ignorant of the concrete type.
type Store[T any] struct {
	mu    sync.RWMutex
	items map[string]T
	keyOf func(T) string
}

func NewStore[T any](keyOf func(T) string) *Store[T] {
	return &Store[T]{items: make(map[string]T), keyOf: keyOf}
}

func (s *Store[T]) Save(t T) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[s.keyOf(t)] = t
	return nil
}

func (s *Store[T]) FindByID(id string) (T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.items[id]
	if !ok {
		var zero T
		return zero, &store.ErrNotFound{ID: id}
	}
	return t, nil
}

func (s *Store[T]) FindAll() ([]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]T, 0, len(s.items))
	for _, v := range s.items {
		out = append(out, v)
	}
	return out, nil
}
