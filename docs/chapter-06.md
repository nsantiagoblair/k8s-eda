# Chapter 6 — Service Layer

**Branch:** `06-service-layer`

## What we're building

No new features. Chapter 6 is a refactoring chapter — we tidy the codebase so it can support multiple entities cleanly in Chapter 7.

Three changes land simultaneously because they are closely related:

1. **`internal/` layout** — all packages move under `internal/`, signalling they are private to this module
2. **Generic `Store[T]`** — one interface and one in-memory implementation that works for any entity
3. **Application service layer** — a `service.TradeService` that sits between the HTTP handler and the store, centralising business logic

The `provider/` package (dead code since Chapter 5) is also deleted.

---

## New concepts

### The `internal/` directory

Go has a special rule: a package under `internal/` can only be imported by code rooted at the parent of the `internal/` directory.

```
github.com/nsantiagoblair/k8s-eda/
├── internal/
│   ├── trade/    ← can only be imported by code in this module
│   ├── store/
│   └── ...
└── cmd/
    ├── trade-api/    ← can import internal/ ✓
    └── provider-stub/  ← can import internal/ ✓
```

If someone tried to add this module as a dependency and import `internal/trade`, the Go compiler would refuse. `internal/` packages are an implementation detail, not a public API.

For a single-module project like ours the enforcement is less visible, but the convention is still valuable: it signals to readers that these packages are not intended for external consumption, and it keeps the option open to extract them as a library later without breaking external callers.

### Go Generics — `Store[T]`

Before Chapter 6, the `Store` interface was tied to `*trade.Trade`:

```go
type Store interface {
    Save(t *Trade) error
    FindByID(id string) (*Trade, error)
    FindAll() ([]*Trade, error)
}
```

In Chapter 7 we add `Order` and `Portfolio` entities. Without generics we'd copy-paste this interface three times. With generics:

```go
// internal/store/store.go
type Store[T any] interface {
    Save(t T) error
    FindByID(id string) (T, error)
    FindAll() ([]T, error)
}
```

`T` is a **type parameter** — a placeholder filled in at compile time. `Store[*trade.Trade]` and `Store[*order.Order]` are different types derived from the same definition.

#### `InMemoryStore[T]`

```go
type InMemoryStore[T any] struct {
    mu    sync.RWMutex
    items map[string]T
    keyOf func(T) string   // caller provides ID extraction
}

func NewInMemoryStore[T any](keyOf func(T) string) *InMemoryStore[T] {
    return &InMemoryStore[T]{items: make(map[string]T), keyOf: keyOf}
}
```

`keyOf` is the trick: instead of requiring `T` to implement an `Identifiable` interface (which would force every entity to have a method), we accept the extraction logic as a plain function. The caller knows the type; the store stays ignorant:

```go
// In cmd/trade-api/main.go:
store.NewInMemoryStore(func(t *trade.Trade) string { return t.ID })

// In Chapter 7 for orders:
store.NewInMemoryStore(func(o *order.Order) string { return o.ID })
```

#### The zero value problem

When `FindByID` returns "not found", it must return a zero value for `T`:

```go
func (s *InMemoryStore[T]) FindByID(id string) (T, error) {
    t, ok := s.items[id]
    if !ok {
        var zero T   // for pointer types like *trade.Trade, this is nil
        return zero, &ErrNotFound{ID: id}
    }
    return t, nil
}
```

`var zero T` gives the zero value of the type parameter at compile time — `nil` for pointer types, `0` for numbers, `""` for strings.

#### Generic functions

`broker.HandlerFunc[T]` is a **generic function** (not a generic type):

```go
func HandlerFunc[T any](fn func(context.Context, T) error) Handler {
    return func(ctx context.Context, msg []byte) error {
        var v T
        if err := json.Unmarshal(msg, &v); err != nil {
            return fmt.Errorf("unmarshal %T: %w", v, err)
        }
        return fn(ctx, v)
    }
}
```

It wraps any typed function as a `broker.Handler`, handling JSON unmarshalling automatically. What used to be this:

```go
func (c *TradeConsumer) HandleFulfilled(ctx context.Context, msg []byte) error {
    var e event.TradeFulfilled
    if err := json.Unmarshal(msg, &e); err != nil {
        return fmt.Errorf("unmarshal TradeFulfilled: %w", err)
    }
    return c.applyTransition(e.TradeID, trade.Fulfilled)
}
```

...becomes this:

```go
func FulfilledHandler(svc tradeService) broker.Handler {
    return broker.HandlerFunc[event.TradeFulfilled](func(_ context.Context, e event.TradeFulfilled) error {
        return svc.Fulfill(e.TradeID)
    })
}
```

The `json.Unmarshal` boilerplate is gone. The function body is pure business logic.

### Application service layer

Before Chapter 6, business logic was split between the HTTP handler and domain methods. The handler knew about stores, transitions, and event publishing:

```go
// handler — too much responsibility
func (h *TradeHandler) submit(w http.ResponseWriter, r *http.Request) {
    t, _ := h.store.FindByID(id)
    t.Transition(trade.Submitted)
    h.store.Save(t)
    h.publisher.Publish(ctx, event.TopicTradeSubmitted, evt)
    writeJSON(w, http.StatusAccepted, t)
}
```

After Chapter 6, `TradeService` owns all of that logic:

```go
// service — all logic in one place
func (s *TradeService) Submit(ctx context.Context, id string) (*trade.Trade, error) {
    t, err := s.store.FindByID(id)
    ...
    t.Transition(trade.Submitted)
    s.store.Save(t)
    s.pub.Publish(ctx, event.TopicTradeSubmitted, evt)
    return t, nil
}

// handler — just parsing and status codes
func (h *TradeHandler) submit(w http.ResponseWriter, r *http.Request) {
    t, err := h.service.Submit(r.Context(), r.PathValue("id"))
    if err != nil {
        writeError(w, httpStatus(err), err.Error())
        return
    }
    writeJSON(w, http.StatusAccepted, t)
}
```

The handler only maps errors to HTTP status codes and writes the response. The service is now independently testable without any HTTP machinery.

#### Consumer-defined interfaces

The handler defines its own `tradeService` interface:

```go
// internal/handler/trade.go
type tradeService interface {
    Create(asset string, side trade.Side, qty int, price float64) (*trade.Trade, error)
    Submit(ctx context.Context, id string) (*trade.Trade, error)
    GetByID(id string) (*trade.Trade, error)
    List() ([]*trade.Trade, error)
}
```

`service.TradeService` satisfies this interface implicitly — the handler doesn't import `service` at all. This is the Go proverb in practice: **"Accept interfaces, return concrete types."** The handler asks for the minimum it needs; `TradeService` provides it without knowing.

The same pattern in `consumer/trade.go`:

```go
type tradeService interface {
    Fulfill(tradeID string) error
    Reject(tradeID string) error
}
```

Two completely different views of the same service, each defined by who uses it.

### Typed errors — `ErrInvalidTransition`

Chapter 5's `Transition` returned a plain string error. Chapter 6 introduces a typed error:

```go
type ErrInvalidTransition struct {
    From Status
    To   Status
}
```

The HTTP handler now has a clean way to map it to 409 Conflict:

```go
func httpStatus(err error) int {
    var notFound *store.ErrNotFound
    if errors.As(err, &notFound) { return http.StatusNotFound }

    var invalidTransition *trade.ErrInvalidTransition
    if errors.As(err, &invalidTransition) { return http.StatusConflict }

    return http.StatusInternalServerError
}
```

`errors.As` unwraps the error chain — it still works if the typed error is wrapped inside another error (`fmt.Errorf("...: %w", err)`).

---

## Final package layout

```
internal/
  trade/            ← domain model: Trade, Side, Status, ErrInvalidTransition
  store/            ← Store[T] interface, ErrNotFound, InMemoryStore[T], SQLiteStore
  service/          ← TradeService — all business logic
  handler/          ← HTTP handlers (thin, delegates to service)
  consumer/         ← Kafka handlers (thin, delegates to service)
  broker/           ← Publisher/Consumer interfaces, HandlerFunc[T], Kafka + memory impls
  event/            ← domain event types and topic constants
cmd/
  trade-api/        ← wires everything together, starts HTTP + consumers
  provider-stub/    ← consumes TradeSubmitted, publishes outcome events
```

This is the Hexagonal Architecture pattern applied fully:
- **Domain** (`trade/`, `event/`) — pure business concepts, no infrastructure imports
- **Ports** (`store.Store[T]`, `broker.Publisher`, `broker.Consumer`) — interfaces the domain uses
- **Adapters** (`store/sqlite.go`, `broker/kafka.go`, `handler/`, `consumer/`) — concrete implementations of the ports
- **Application** (`service/`) — orchestrates domain objects through ports

---

## Testing

### What the new tests cover

| Package | Test | What it checks |
|---------|------|----------------|
| `internal/store` | `TestInMemoryStore_Generic` | `InMemoryStore[T]` works with a non-trade type |
| `internal/broker` | `TestHandlerFunc` | Typed handler receives correctly unmarshalled value |
| `internal/broker` | `TestHandlerFunc_InvalidJSON` | Unmarshal error is returned |
| `internal/service` | `TestCreate`, `TestSubmit_*` | Service validates input, publishes events, returns typed errors |
| `internal/service` | `TestFulfill`, `TestReject` | Status transitions work via the service |
| `internal/handler` | All existing tests | Updated to use service layer |
| `internal/consumer` | `TestFulfilledHandler`, `TestRejectedHandler` | Consumers delegate to service correctly |

### Testing the service layer independently

Because `TradeService` takes `store.Store[*trade.Trade]` (an interface) and `broker.Publisher` (an interface), tests can inject an `InMemoryStore` and `InMemoryPublisher` without any HTTP or Kafka machinery:

```go
s := store.NewInMemoryStore(func(t *trade.Trade) string { return t.ID })
pub := broker.NewInMemoryPublisher()
svc := service.NewTradeService(s, pub)

tr, _ := svc.Create("AAPL", trade.Buy, 10, 195.00)
svc.Submit(context.Background(), tr.ID)

// Assert event was published
if pub.Count(event.TopicTradeSubmitted) != 1 { ... }
```

This is the payoff of the architecture: each layer is testable in complete isolation.

---

## Manual testing

Manual testing is identical to Chapter 5 — `docker compose up -d`, then run both binaries. Nothing changed externally. This is the point: a complete internal restructure with no visible behaviour change, and all tests still green.

```bash
docker compose up -d
go run ./cmd/trade-api
go run ./cmd/provider-stub

curl -s -X POST http://localhost:8080/trades \
  -H "Content-Type: application/json" \
  -d '{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}' | jq .id

curl -s -X POST http://localhost:8080/trades/<id>/submit | jq .status
# SUBMITTED — poll until FULFILLED
curl -s http://localhost:8080/trades/<id> | jq .status
# FULFILLED
```

---

## Coming in Chapter 7

Chapter 7 adds `Order` and `Portfolio` entities. Each gets:
- A domain struct in `internal/order/` and `internal/portfolio/`
- A `store.NewInMemoryStore(...)` instance (generic — no new interface needed)
- A service (following the same pattern as `TradeService`)
- Its own Kafka consumer

The `Store[T]` and `HandlerFunc[T]` generics introduced here mean each new entity needs almost no boilerplate.