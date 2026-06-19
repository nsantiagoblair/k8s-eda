package store_test

import (
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/store"
	"github.com/nsantiagoblair/k8s-eda/trade"
)

func TestInMemoryStore_SaveAndFindByID(t *testing.T) {
	s := store.NewInMemoryStore()

	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	if err := s.Save(tr); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	found, err := s.FindByID(tr.ID)
	if err != nil {
		t.Fatalf("FindByID() unexpected error: %v", err)
	}
	if found.ID != tr.ID {
		t.Errorf("expected ID %s, got %s", tr.ID, found.ID)
	}
}

func TestInMemoryStore_Save_Overwrites(t *testing.T) {
	s := store.NewInMemoryStore()

	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	s.Save(tr) //nolint:errcheck

	tr.Status = trade.Submitted
	s.Save(tr) //nolint:errcheck

	found, _ := s.FindByID(tr.ID)
	if found.Status != trade.Submitted {
		t.Errorf("expected status SUBMITTED after overwrite, got %s", found.Status)
	}
}

func TestInMemoryStore_FindByID_NotFound(t *testing.T) {
	s := store.NewInMemoryStore()

	_, err := s.FindByID("does-not-exist")

	var notFound *trade.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestInMemoryStore_FindAll(t *testing.T) {
	s := store.NewInMemoryStore()

	if trades, _ := s.FindAll(); len(trades) != 0 {
		t.Errorf("expected empty store, got %d trades", len(trades))
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

func TestInMemoryStore_FindByStatus(t *testing.T) {
	s := store.NewInMemoryStore()

	t1, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	t2, _ := trade.New("TSLA", trade.Buy, 5, 250.00)
	t2.Transition(trade.Submitted) //nolint:errcheck
	s.Save(t1)                     //nolint:errcheck
	s.Save(t2)                     //nolint:errcheck

	pending, _ := s.FindByStatus(trade.Pending)
	if len(pending) != 1 {
		t.Errorf("expected 1 pending trade, got %d", len(pending))
	}

	submitted, _ := s.FindByStatus(trade.Submitted)
	if len(submitted) != 1 {
		t.Errorf("expected 1 submitted trade, got %d", len(submitted))
	}

	fulfilled, _ := s.FindByStatus(trade.Fulfilled)
	if len(fulfilled) != 0 {
		t.Errorf("expected 0 fulfilled trades, got %d", len(fulfilled))
	}
}
