package memory_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/infra/memory"
	"github.com/nsantiagoblair/k8s-eda/internal/store"
)

// --- Store[T] ---------------------------------------------------------------

func TestStore_SaveAndFind(t *testing.T) {
	type item struct{ ID, Name string }
	s := memory.NewStore(func(i *item) string { return i.ID })

	s.Save(&item{ID: "1", Name: "Widget"}) //nolint:errcheck
	got, err := s.FindByID("1")
	if err != nil { t.Fatalf("FindByID: %v", err) }
	if got.Name != "Widget" { t.Errorf("Name = %s; want Widget", got.Name) }
}

func TestStore_FindByID_NotFound(t *testing.T) {
	s := memory.NewStore(func(i *struct{ ID string }) string { return i.ID })
	_, err := s.FindByID("missing")
	var notFound *store.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrNotFound, got %T", err)
	}
}

func TestStore_FindAll(t *testing.T) {
	type item struct{ ID string }
	s := memory.NewStore(func(i *item) string { return i.ID })
	s.Save(&item{ID: "a"}) //nolint:errcheck
	s.Save(&item{ID: "b"}) //nolint:errcheck
	all, _ := s.FindAll()
	if len(all) != 2 { t.Errorf("FindAll = %d; want 2", len(all)) }
}

// --- Publisher --------------------------------------------------------------

func TestPublisher_CountAndMessages(t *testing.T) {
	pub := memory.NewPublisher()
	ctx := context.Background()

	pub.Publish(ctx, "t1", map[string]string{"x": "1"}) //nolint:errcheck
	pub.Publish(ctx, "t1", map[string]string{"x": "2"}) //nolint:errcheck
	pub.Publish(ctx, "t2", map[string]string{"x": "3"}) //nolint:errcheck

	if pub.Count("t1") != 2 { t.Errorf("Count(t1) = %d; want 2", pub.Count("t1")) }
	if pub.Count("t2") != 1 { t.Errorf("Count(t2) = %d; want 1", pub.Count("t2")) }
	if pub.Count("t3") != 0 { t.Errorf("Count(t3) = %d; want 0", pub.Count("t3")) }
}
