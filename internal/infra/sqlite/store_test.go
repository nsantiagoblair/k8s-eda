package sqlite_test

import (
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/infra/sqlite"
	"github.com/nsantiagoblair/k8s-eda/internal/store"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

func newTestStore(t *testing.T) *sqlite.Store {
	t.Helper()
	s, err := sqlite.NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteStore_SaveAndFindByID(t *testing.T) {
	s := newTestStore(t)
	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	s.Save(tr) //nolint:errcheck
	got, err := s.FindByID(tr.ID)
	if err != nil { t.Fatalf("FindByID: %v", err) }
	if got.Asset != "AAPL" { t.Errorf("Asset = %s; want AAPL", got.Asset) }
}

func TestSQLiteStore_FindByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.FindByID("no-such-id")
	var notFound *store.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestSQLiteStore_Save_UpdatesStatus(t *testing.T) {
	s := newTestStore(t)
	tr, _ := trade.New("AAPL", trade.Buy, 10, 195.00)
	s.Save(tr) //nolint:errcheck
	tr.Transition(trade.Submitted) //nolint:errcheck
	s.Save(tr) //nolint:errcheck
	got, _ := s.FindByID(tr.ID)
	if got.Status != trade.Submitted { t.Errorf("Status = %s; want SUBMITTED", got.Status) }
}

func TestSQLiteStore_FindAll(t *testing.T) {
	s := newTestStore(t)
	t1, _ := trade.New("AAPL", trade.Buy,  10, 195.00)
	t2, _ := trade.New("TSLA", trade.Sell, 5,  250.00)
	s.Save(t1) //nolint:errcheck
	s.Save(t2) //nolint:errcheck
	all, err := s.FindAll()
	if err != nil { t.Fatalf("FindAll: %v", err) }
	if len(all) != 2 { t.Errorf("FindAll = %d; want 2", len(all)) }
}
