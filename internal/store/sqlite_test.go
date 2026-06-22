package store_test

import (
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/store"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

func newTestSQLiteStore(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLiteStore(":memory:")
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteStore_SaveAndFindByID(t *testing.T) {
	s := newTestSQLiteStore(t)
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

func TestSQLiteStore_FindByID_NotFound(t *testing.T) {
	s := newTestSQLiteStore(t)
	_, err := s.FindByID("no-such-id")

	var notFound *store.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestSQLiteStore_Save_UpdatesStatus(t *testing.T) {
	s := newTestSQLiteStore(t)
	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	s.Save(tr) //nolint:errcheck

	tr.Transition(trade.Submitted) //nolint:errcheck
	s.Save(tr)                     //nolint:errcheck

	got, _ := s.FindByID(tr.ID)
	if got.Status != trade.Submitted {
		t.Errorf("Status = %s; want SUBMITTED", got.Status)
	}
}

func TestSQLiteStore_FindAll(t *testing.T) {
	s := newTestSQLiteStore(t)
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

func TestSQLiteStore_RoundTripsAllFields(t *testing.T) {
	s := newTestSQLiteStore(t)
	tr, _ := trade.New("NVDA", trade.Sell, 3, 890.50)
	s.Save(tr) //nolint:errcheck

	got, _ := s.FindByID(tr.ID)
	if got.ID != tr.ID               { t.Errorf("ID mismatch") }
	if got.Asset != tr.Asset         { t.Errorf("Asset mismatch") }
	if got.Side != tr.Side           { t.Errorf("Side mismatch") }
	if got.Quantity != tr.Quantity   { t.Errorf("Quantity mismatch") }
	if got.LimitPrice != tr.LimitPrice { t.Errorf("LimitPrice mismatch") }
	if got.Status != tr.Status       { t.Errorf("Status mismatch") }
}
