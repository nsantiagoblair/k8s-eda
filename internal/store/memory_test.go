package store_test

import (
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/store"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

func newTradeStore() *store.InMemoryStore[*trade.Trade] {
	return store.NewInMemoryStore(func(t *trade.Trade) string { return t.ID })
}

func TestInMemoryStore_SaveAndFind(t *testing.T) {
	s := newTradeStore()
	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)

	if err := s.Save(tr); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := s.FindByID(tr.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Asset != "AAPL" {
		t.Errorf("Asset = %s; want AAPL", got.Asset)
	}
}

func TestInMemoryStore_FindByID_NotFound(t *testing.T) {
	s := newTradeStore()
	_, err := s.FindByID("no-such-id")

	var notFound *store.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("expected ErrNotFound, got %T: %v", err, err)
	}
	if notFound.ID != "no-such-id" {
		t.Errorf("ErrNotFound.ID = %s; want no-such-id", notFound.ID)
	}
}

func TestInMemoryStore_Save_Overwrites(t *testing.T) {
	s := newTradeStore()
	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	s.Save(tr) //nolint:errcheck

	tr.Transition(trade.Submitted) //nolint:errcheck
	s.Save(tr)                     //nolint:errcheck

	got, _ := s.FindByID(tr.ID)
	if got.Status != trade.Submitted {
		t.Errorf("Status = %s; want SUBMITTED", got.Status)
	}
}

func TestInMemoryStore_FindAll(t *testing.T) {
	s := newTradeStore()
	t1, _ := trade.New("AAPL", trade.Buy,  10, 195.00)
	t2, _ := trade.New("TSLA", trade.Sell, 5,  250.00)
	s.Save(t1) //nolint:errcheck
	s.Save(t2) //nolint:errcheck

	all, err := s.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("FindAll = %d items; want 2", len(all))
	}
}

// TestInMemoryStore_Generic demonstrates that InMemoryStore works for any type,
// not just *trade.Trade.
func TestInMemoryStore_Generic(t *testing.T) {
	type Product struct {
		SKU  string
		Name string
	}

	s := store.NewInMemoryStore(func(p *Product) string { return p.SKU })
	s.Save(&Product{SKU: "abc-123", Name: "Widget"}) //nolint:errcheck

	got, err := s.FindByID("abc-123")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if got.Name != "Widget" {
		t.Errorf("Name = %s; want Widget", got.Name)
	}
}
