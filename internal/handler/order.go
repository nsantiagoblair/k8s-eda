package handler

import (
	"net/http"

	"github.com/nsantiagoblair/k8s-eda/internal/order"
)

// orderService is the read-only view of service.OrderService the handler needs.
// Orders are created and updated via Kafka events — the HTTP layer only exposes reads.
type orderService interface {
	GetByID(id string) (*order.Order, error)
	List() ([]*order.Order, error)
}

// OrderHandler exposes read-only HTTP endpoints for orders.
type OrderHandler struct {
	service orderService
}

func NewOrderHandler(svc orderService) *OrderHandler {
	return &OrderHandler{service: svc}
}

func (h *OrderHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /orders/{id}", h.getByID)
	mux.HandleFunc("GET /orders", h.list)
}

func (h *OrderHandler) getByID(w http.ResponseWriter, r *http.Request) {
	o, err := h.service.GetByID(r.PathValue("id"))
	if err != nil {
		writeError(w, httpStatus(err), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (h *OrderHandler) list(w http.ResponseWriter, r *http.Request) {
	orders, err := h.service.List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list orders")
		return
	}
	writeJSON(w, http.StatusOK, orders)
}
