package consumer_test

import (
	"context"
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/consumer"
	"github.com/nsantiagoblair/k8s-eda/internal/event"
	"github.com/nsantiagoblair/k8s-eda/internal/infra/memory"
	"github.com/nsantiagoblair/k8s-eda/internal/service"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"
)

func newSubmittedTrade(t *testing.T) (*service.TradeService, *trade.Trade) {
	t.Helper()
	s := memory.NewStore(func(tr *trade.Trade) string { return tr.ID })
	svc := service.NewTradeService(s, memory.NewPublisher())
	tr, _ := svc.Create("AAPL", trade.Buy, 10, 150.00)
	svc.Submit(context.Background(), tr.ID) //nolint:errcheck
	return svc, tr
}

func TestFulfilledHandler(t *testing.T) {
	svc, tr := newSubmittedTrade(t)
	h := consumer.FulfilledHandler(svc)

	pub := memory.NewPublisher()
	pub.Publish(context.Background(), "t", event.TradeFulfilled{TradeID: tr.ID}) //nolint:errcheck
	payload := pub.Messages("t")[0]

	if err := h(context.Background(), payload); err != nil {
		t.Fatalf("FulfilledHandler: %v", err)
	}

	got, _ := svc.GetByID(tr.ID)
	if got.Status != trade.Fulfilled {
		t.Errorf("Status = %s; want FULFILLED", got.Status)
	}
}

func TestRejectedHandler(t *testing.T) {
	svc, tr := newSubmittedTrade(t)
	h := consumer.RejectedHandler(svc)

	pub := memory.NewPublisher()
	pub.Publish(context.Background(), "t", event.TradeRejected{TradeID: tr.ID}) //nolint:errcheck
	payload := pub.Messages("t")[0]

	if err := h(context.Background(), payload); err != nil {
		t.Fatalf("RejectedHandler: %v", err)
	}

	got, _ := svc.GetByID(tr.ID)
	if got.Status != trade.Rejected {
		t.Errorf("Status = %s; want REJECTED", got.Status)
	}
}
