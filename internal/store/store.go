// Package store defines the generic repository interface and shared error types.
package store

import "fmt"

// Store is a generic repository for any entity identified by a string ID.
// The type parameter T is constrained to any type — typically a pointer to a
// domain struct (e.g. *trade.Trade, *order.Order).
//
// Using generics here means we write the interface once and satisfy it with
// different backing stores (in-memory, SQLite, Postgres) for any entity, with
// no code duplication and full type safety.
type Store[T any] interface {
	Save(t T) error
	FindByID(id string) (T, error)
	FindAll() ([]T, error)
}

// ErrNotFound is returned when an entity with the requested ID does not exist.
// Using a typed error (rather than a plain string) lets callers distinguish
// "not found" from other errors with errors.As, and map it to a 404 response
// without string matching.
type ErrNotFound struct {
	ID string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("not found: %s", e.ID)
}
