package trade

import "fmt"

// Store defines the operations for persisting and retrieving trades.
//
// Defining behaviour as an interface means we can swap the implementation
// (in-memory → SQLite → Postgres) without changing any code that depends on it.
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
