# Chapter 7 — Multi-Entity

**Branch:** `07-multi-entity`

## What we're building

Two changes land in this chapter:

1. **`infra/` layout** — implementation packages move under `internal/infra/`, giving the codebase clear, visible architectural boundaries
2. **`Order` entity** — a second domain entity added to demonstrate that `Store[T]` and `HandlerFunc[T]` from Chapter 6 eliminate boilerplate for every entity beyond the first

---

## The `infra/` refactor — why and what changed

After Chapter 6 every package lived flat under `internal/`. The boundary between "what the domain defines" and "what talks to an external system" was clear from the code but not from the folder structure.

```
Before (flat):
internal/
  store/      ← Store[T] interface AND InMemoryStore[T] AND SQLiteStore
  broker/     ← Publisher/Consumer interfaces AND KafkaPublisher AND InMemoryPublisher
```

```
After (separated):
internal/
  store/      ← Store[T] interface and ErrNotFound only   (port)
  broker/     ← Publisher/Consumer/Handler interfaces only (port)
  infra/
    sqlite/   ← SQLiteStore                               (adapter)
    kafka/    ← KafkaPublisher, KafkaConsumer             (adapter)
    memory/   ← InMemoryStore[T], InMemoryPublisher       (test doubles)
```

The rule is now explicit and enforced by the directory structure:
- **`store/` and `broker/`** — contain only interfaces (ports). Zero external dependencies.
- **`infra/`** — contains everything that touches an external system. If you see an `infra/` import, you know there is real I/O involved.
- **`infra/memory/`** — in-memory implementations with no I/O. Used in tests and as a default before a persistent store is available.

### Package naming in `infra/`

Types are named after their role, not their technology:

```go
sqlite.Store    // not SQLiteStore — the package name provides context
kafka.Publisher // not KafkaPublisher
memory.Store[T] // not InMemoryStore[T]
```

Callers are unambiguous: `sqlite.NewStore(...)`, `kafka.NewPublisher(...)`, `memory.NewStore(...)`.

### Multiple consumer groups on the same topic

`cmd/trade-api/main.go` now starts two independent consumer groups reading from the same topics:

```go
// Group "trade-api-trades" — updates Trade status
startConsumer(ctx, brokers, event.TopicTradeFulfilled, "trade-api-trades", consumer.FulfilledHandler(tradeSvc))
startConsumer(ctx, brokers, event.TopicTradeRejected,  "trade-api-trades", consumer.RejectedHandler(tradeSvc))

// Group "trade-api-orders" — updates Order status
startConsumer(ctx, brokers, event.TopicTradeSubmitted,  "trade-api-orders", consumer.OrderCreatedHandler(orderSvc))
startConsumer(ctx, brokers, event.TopicTradeFulfilled,  "trade-api-orders", consumer.OrderFilledHandler(orderSvc))
startConsumer(ctx, brokers, event.TopicTradeRejected,   "trade-api-orders", consumer.OrderCancelledHandler(orderSvc))
```

Kafka delivers each message to every consumer group independently. The `trade-api-trades` and `trade-api-orders` groups both receive every `TradeFulfilled` message and process it for their own purpose. Neither group knows the other exists.

This is the fan-out pattern — one event, multiple consumers, no coordination needed.

---

## The `Order` entity

### Trade vs Order — why two entities?

A **Trade** is an instruction from a user: "buy 10 shares of AAPL at ≤ $195". It is a user-facing concept.

An **Order** is a record of what was actually sent to the market on the user's behalf. In a real system these diverge:
- An order can be partially filled
- A single trade might result in multiple orders (order splitting)
- Orders carry execution-specific data the user doesn't care about (fill price, timestamps)

For now they have a 1:1 relationship, but keeping them separate is the correct domain model.

### Order lifecycle

```
TradeSubmitted event arrives
       │
       ▼
   Order created (PENDING)
       │
   ┌───┴──────────────┐
   ▼                  ▼
TradeFulfilled    TradeRejected
   event              event
   │                  │
   ▼                  ▼
 FILLED           CANCELLED
```

### `Order` status transitions

```go
var validTransitions = map[Status][]Status{
    Pending:   {Filled, Cancelled},
    Filled:    {},
    Cancelled: {},
}
```

The `ErrInvalidTransition` typed error follows the same pattern as in `trade`.

### Adding a second entity — the payoff of Chapter 6

Compare what was needed to add `Order` vs what would have been needed in Chapter 5:

**Chapter 5 (before generics/service layer):**
- New `Store` interface specific to `*Order`
- New `InMemoryOrderStore` struct with all CRUD methods
- Direct handler → store wiring (no service layer to extend)
- Manual `json.Unmarshal` in each consumer handler

**Chapter 7 (with generics/service layer):**
- Domain struct (`order.go`) — unavoidable, it's the entity
- Service (`service/order.go`) — clean, follows the same pattern as `TradeService`
- Consumer handlers (`consumer/order.go`) — three one-liners using `broker.HandlerFunc[T]`
- HTTP handler (`handler/order.go`) — thin read-only handler

No new store interface. No new store implementation. No unmarshal boilerplate. `Store[T]` and `HandlerFunc[T]` absorbed the new entity with minimal code.

---

## Complete package layout

```
internal/
  trade/            ← domain: Trade, ErrInvalidTransition
  order/            ← domain: Order, ErrInvalidTransition (new)
  event/            ← domain: event types, topic constants
  store/            ← port: Store[T] interface, ErrNotFound
  broker/           ← port: Publisher, Consumer, Handler, HandlerFunc[T]
  service/
    trade.go        ← TradeService
    order.go        ← OrderService (new)
  handler/
    trade.go        ← trade HTTP endpoints
    order.go        ← order HTTP endpoints (new, read-only)
    http.go         ← shared writeJSON, writeError, httpStatus
  consumer/
    trade.go        ← FulfilledHandler, RejectedHandler
    order.go        ← OrderCreatedHandler, OrderFilledHandler, OrderCancelledHandler (new)
  infra/            ← (new) outbound adapters
    sqlite/         ← sqlite.Store (trade-specific)
    kafka/          ← kafka.Publisher, kafka.Consumer
    memory/         ← memory.Store[T], memory.Publisher
cmd/
  trade-api/
  provider-stub/
```

Boundaries at a glance:
- **No `infra/` import in domain, service, handler, consumer** — those layers are I/O free
- **`infra/` imports domain** (e.g. `sqlite` imports `internal/trade`) — adapters depend on domain, not the other way round
- **`cmd/` imports `infra/`** — wiring only happens at the entry point

---

## Testing

### What's new

| Package | Test | What it checks |
|---------|------|----------------|
| `infra/memory` | `TestStore_*`, `TestPublisher_*` | Moved from `internal/store`, `internal/broker` |
| `infra/sqlite` | `TestSQLiteStore_*` | Moved from `internal/store` |
| `order` | `TestNew`, `TestTransition` | Domain model behaves like `trade` |
| `service` | `TestOrderService_*` | Create, fill, cancel via TradeID |

### `findByTradeID` — O(n) scan

`OrderService.findByTradeID` scans all orders to find one by `TradeID`:

```go
func (s *OrderService) findByTradeID(tradeID string) (*order.Order, error) {
    all, _ := s.store.FindAll()
    for _, o := range all {
        if o.TradeID == tradeID { return o, nil }
    }
    return nil, &store.ErrNotFound{ID: tradeID}
}
```

This is fine with `memory.Store[T]` (all in RAM) and acceptable for a tutorial. In Chapter 9, when we add Postgres, this becomes a SQL query with an index:

```sql
SELECT * FROM orders WHERE trade_id = $1
```

The O(n) scan is a deliberate simplification — the important point now is the pattern, not the performance.

---

## Manual testing

```bash
docker compose up -d
go run ./cmd/trade-api
go run ./cmd/provider-stub
```

### Create and submit a trade

```bash
ID=$(curl -s -X POST http://localhost:8080/trades \
  -H "Content-Type: application/json" \
  -d '{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}' | jq -r .id)

curl -s -X POST http://localhost:8080/trades/$ID/submit | jq .status
# SUBMITTED
```

### Check the order was created

```bash
curl -s http://localhost:8080/orders | jq .
# [{...,"tradeId":"<ID>","status":"PENDING",...}]
```

### Wait for fulfilment, then check both

```bash
# After ~1 second the provider processes the event
curl -s http://localhost:8080/trades/$ID | jq .status
# FULFILLED

curl -s http://localhost:8080/orders | jq '.[0].status'
# FILLED
```

Both the trade and its order updated independently, each via its own consumer group, from the same Kafka events.

### Test the rejection path

```bash
LOW_ID=$(curl -s -X POST http://localhost:8080/trades \
  -H "Content-Type: application/json" \
  -d '{"asset":"GME","side":"BUY","quantity":100,"limitPrice":5.00}' | jq -r .id)

curl -s -X POST http://localhost:8080/trades/$LOW_ID/submit > /dev/null
sleep 1

curl -s http://localhost:8080/trades/$LOW_ID | jq .status   # REJECTED
curl -s http://localhost:8080/orders | jq '.[] | select(.tradeId == "'$LOW_ID'") | .status'   # CANCELLED
```

---

## Coming in Chapter 8

Chapter 8 adds a **Portfolio** entity that tracks a user's holdings. Each time a trade is fulfilled, the portfolio is updated. This introduces the concept of **aggregate state** — the portfolio is derived from a stream of fulfilled trades, which sets up Chapter 10's discussion of event sourcing.
