package service_test

import (
	"testing"

	"github.com/nsantiagoblair/k8s-eda/internal/event"
	"github.com/nsantiagoblair/k8s-eda/internal/infra/memory"
	"github.com/nsantiagoblair/k8s-eda/internal/order"
	"github.com/nsantiagoblair/k8s-eda/internal/service"
)

func newOrderSvc() *service.OrderService {
	s := memory.NewStore(func(o *order.Order) string { return o.ID })
	return service.NewOrderService(s)
}

func TestOrderService_CreateFromTrade(t *testing.T) {
	svc := newOrderSvc()
	evt := event.TradeSubmitted{TradeID: "t1", Asset: "AAPL", Side: "BUY", Quantity: 10, LimitPrice: 195.00}

	if err := svc.CreateFromTrade(evt); err != nil {
		t.Fatalf("CreateFromTrade: %v", err)
	}

	orders, _ := svc.List()
	if len(orders) != 1 { t.Fatalf("expected 1 order, got %d", len(orders)) }
	if orders[0].TradeID != "t1" { t.Errorf("TradeID = %s; want t1", orders[0].TradeID) }
	if orders[0].Status != order.Pending { t.Errorf("Status = %s; want PENDING", orders[0].Status) }
}

func TestOrderService_FillByTradeID(t *testing.T) {
	svc := newOrderSvc()
	svc.CreateFromTrade(event.TradeSubmitted{TradeID: "t1", Asset: "AAPL", Side: "BUY", Quantity: 1, LimitPrice: 100}) //nolint:errcheck

	if err := svc.FillByTradeID("t1"); err != nil {
		t.Fatalf("FillByTradeID: %v", err)
	}

	orders, _ := svc.List()
	if orders[0].Status != order.Filled { t.Errorf("Status = %s; want FILLED", orders[0].Status) }
}

func TestOrderService_CancelByTradeID(t *testing.T) {
	svc := newOrderSvc()
	svc.CreateFromTrade(event.TradeSubmitted{TradeID: "t1", Asset: "AAPL", Side: "BUY", Quantity: 1, LimitPrice: 100}) //nolint:errcheck

	if err := svc.CancelByTradeID("t1"); err != nil {
		t.Fatalf("CancelByTradeID: %v", err)
	}

	orders, _ := svc.List()
	if orders[0].Status != order.Cancelled { t.Errorf("Status = %s; want CANCELLED", orders[0].Status) }
}

func TestOrderService_FillByTradeID_NotFound(t *testing.T) {
	svc := newOrderSvc()
	if err := svc.FillByTradeID("no-such-trade"); err == nil {
		t.Error("expected error for unknown trade ID")
	}
}
