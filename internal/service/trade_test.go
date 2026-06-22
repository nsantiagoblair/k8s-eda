package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/broker"
	"github.com/nsantiagoblair/k8s-eda/internal/event"
	"github.com/nsantiagoblair/k8s-eda/internal/service"
	"github.com/nsantiagoblair/k8s-eda/internal/store"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

func newSvc(t *testing.T) (*service.TradeService, *broker.InMemoryPublisher) {
	t.Helper()
	s := store.NewInMemoryStore(func(tr *trade.Trade) string { return tr.ID })
	pub := broker.NewInMemoryPublisher()
	return service.NewTradeService(s, pub), pub
}

func TestCreate(t *testing.T) {
	svc, _ := newSvc(t)
	tr, err := svc.Create("AAPL", trade.Buy, 10, 195.00)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tr.Status != trade.Pending {
		t.Errorf("Status = %s; want PENDING", tr.Status)
	}
}

func TestCreate_InvalidInput(t *testing.T) {
	svc, _ := newSvc(t)
	if _, err := svc.Create("", trade.Buy, 10, 195.00); err == nil {
		t.Error("expected error for empty asset")
	}
}

func TestSubmit_PublishesEvent(t *testing.T) {
	svc, pub := newSvc(t)
	tr, _ := svc.Create("AAPL", trade.Buy, 10, 195.00)

	submitted, err := svc.Submit(context.Background(), tr.ID)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if submitted.Status != trade.Submitted {
		t.Errorf("Status = %s; want SUBMITTED", submitted.Status)
	}
	if pub.Count(event.TopicTradeSubmitted) != 1 {
		t.Errorf("expected 1 event, got %d", pub.Count(event.TopicTradeSubmitted))
	}
}

func TestSubmit_NotFound(t *testing.T) {
	svc, _ := newSvc(t)
	_, err := svc.Submit(context.Background(), "no-such-id")

	var notFound *store.ErrNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrNotFound, got %T: %v", err, err)
	}
}

func TestSubmit_AlreadySubmitted(t *testing.T) {
	svc, _ := newSvc(t)
	tr, _ := svc.Create("AAPL", trade.Buy, 10, 195.00)
	svc.Submit(context.Background(), tr.ID) //nolint:errcheck

	_, err := svc.Submit(context.Background(), tr.ID)
	var invalidTransition *trade.ErrInvalidTransition
	if !errors.As(err, &invalidTransition) {
		t.Errorf("expected ErrInvalidTransition, got %T: %v", err, err)
	}
}

func TestFulfill(t *testing.T) {
	svc, _ := newSvc(t)
	tr, _ := svc.Create("AAPL", trade.Buy, 10, 195.00)
	svc.Submit(context.Background(), tr.ID) //nolint:errcheck

	if err := svc.Fulfill(tr.ID); err != nil {
		t.Fatalf("Fulfill: %v", err)
	}

	got, _ := svc.GetByID(tr.ID)
	if got.Status != trade.Fulfilled {
		t.Errorf("Status = %s; want FULFILLED", got.Status)
	}
}

func TestReject(t *testing.T) {
	svc, _ := newSvc(t)
	tr, _ := svc.Create("AAPL", trade.Buy, 10, 195.00)
	svc.Submit(context.Background(), tr.ID) //nolint:errcheck

	if err := svc.Reject(tr.ID); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	got, _ := svc.GetByID(tr.ID)
	if got.Status != trade.Rejected {
		t.Errorf("Status = %s; want REJECTED", got.Status)
	}
}
