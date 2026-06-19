package store_test

import (
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/store"
	"github.com/nsantiagoblair/k8s-eda/trade"
)

// newTestStore returns a SQLiteStore backed by an in-memory database.
// Each call produces a completely isolated store — no files, no cleanup needed.
func newTestStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteStore_SaveAndFindByID(t *testing.T) {
	s := newTestStore(t)

	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	if err := s.Save(tr); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	found, err := s.FindByID(tr.ID)
	if err != nil {
		t.Fatalf("FindByID() unexpected error: %v", err)
	}

	if found.ID != tr.ID {
		t.Errorf("ID: expected %s, got %s", tr.ID, found.ID)
	}
	if found.Asset != tr.Asset {
		t.Errorf("Asset: expected %s, got %s", tr.Asset, found.Asset)
	}
	if found.Side != tr.Side {
		t.Errorf("Side: expected %s, got %s", tr.Side, found.Side)
	}
	if found.Quantity != tr.Quantity {
		t.Errorf("Quantity: expected %d, got %d", tr.Quantity, found.Quantity)
	}
	if found.Status != tr.Status {
		t.Errorf("Status: expected %s, got %s", tr.Status, found.Status)
	}
}

func TestSQLiteStore_Save_UpdatesStatus(t *testing.T) {
	s := newTestStore(t)

	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	s.Save(tr) //nolint:errcheck

	tr.Transition(trade.Submitted) //nolint:errcheck
	s.Save(tr)                     //nolint:errcheck

	found, _ := s.FindByID(tr.ID)
	if found.Status != trade.Submitted {
		t.Errorf("expected SUBMITTED after update, got %s", found.Status)
	}
}

func TestSQLiteStore_FindByID_NotFound(t *testing.T) {
	s := newTestStore(t)

	_, err := s.FindByID("does-not-exist")

	var notFound *trade.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestSQLiteStore_FindAll(t *testing.T) {
	s := newTestStore(t)

	trades, _ := s.FindAll()
	if len(trades) != 0 {
		t.Errorf("expected empty result, got %d trades", len(trades))
	}

	t1, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	t2, _ := trade.New("TSLA", trade.Sell, 5, 250.00)
	s.Save(t1) //nolint:errcheck
	s.Save(t2) //nolint:errcheck

	trades, err := s.FindAll()
	if err != nil {
		t.Fatalf("FindAll() unexpected error: %v", err)
	}
	if len(trades) != 2 {
		t.Errorf("expected 2 trades, got %d", len(trades))
	}
}

func TestSQLiteStore_FindByStatus(t *testing.T) {
	s := newTestStore(t)

	t1, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	t2, _ := trade.New("TSLA", trade.Buy, 5, 250.00)
	t2.Transition(trade.Submitted) //nolint:errcheck
	s.Save(t1)                     //nolint:errcheck
	s.Save(t2)                     //nolint:errcheck

	pending, _ := s.FindByStatus(trade.Pending)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}

	submitted, _ := s.FindByStatus(trade.Submitted)
	if len(submitted) != 1 {
		t.Errorf("expected 1 submitted, got %d", len(submitted))
	}

	fulfilled, _ := s.FindByStatus(trade.Fulfilled)
	if len(fulfilled) != 0 {
		t.Errorf("expected 0 fulfilled, got %d", len(fulfilled))
	}
}

func TestSQLiteStore_RoundTripsAllFields(t *testing.T) {
	s := newTestStore(t)

	original, _ := trade.New("NVDA", trade.Sell, 3, 890.50)
	s.Save(original) //nolint:errcheck

	found, _ := s.FindByID(original.ID)

	if !found.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt mismatch: %v vs %v", original.CreatedAt, found.CreatedAt)
	}
	if found.LimitPrice != original.LimitPrice {
		t.Errorf("LimitPrice: expected %.2f, got %.2f", original.LimitPrice, found.LimitPrice)
	}
	if found.Side != trade.Sell {
		t.Errorf("Side: expected SELL, got %s", found.Side)
	}
}
