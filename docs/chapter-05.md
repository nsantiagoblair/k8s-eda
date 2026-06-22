# Chapter 5 — Events

**Branch:** `05-events`

## What we're building

In Chapter 4, the trade API called the provider stub synchronously over HTTP.
That created three problems we now solve:

| Problem | Impact |
|---------|--------|
| Blocking call | Submit handler could take seconds before responding |
| Tight coupling | Trade API needs to know the provider's URL at startup |
| Failure cascade | If the provider is down, every submit request fails immediately |

We replace the HTTP call with **Kafka events**:

```
POST /trades/{id}/submit
       │
       ▼
trade-api  ──[TradeSubmitted]──▶  Kafka  ──▶  provider-stub
                                                     │
trade-api  ◀─[TradeFulfilled / TradeRejected]──  Kafka
```

The submit endpoint returns **202 Accepted** immediately. The outcome arrives later via a background consumer that updates the trade's status in the store.

---

## New concepts

### Kafka fundamentals

Kafka is a distributed log. Producers write **messages** to named **topics**; consumers read them back.

Key ideas to understand:

- **Topic** — an append-only, ordered log. Unlike an HTTP request, messages are stored on disk and can be replayed.
- **Partition** — a topic is split into N partitions for parallelism. Messages within a partition are ordered. We use 1 partition in development.
- **Consumer group** — a named set of consumers that collectively process a topic. Kafka distributes partitions across the group and tracks which offset each group has consumed. If a consumer restarts, it picks up from the last committed offset.
- **Offset** — the position of a message in a partition. Committing an offset is like checking off a task: "I've processed everything up to here."
- **KRaft** — Kafka's built-in consensus mechanism that replaces ZooKeeper. Our Docker Compose file uses it so we need only one container.

### Delivery guarantees

Our implementation commits the offset after processing each message, regardless of whether the handler returned an error:

```go
if err := handler(ctx, msg.Value); err != nil {
    log.Printf("handler error: %v (committing offset anyway)", err)
}
c.reader.CommitMessages(ctx, msg)
```

This is called **at-least-once delivery**: if the service crashes between processing and committing, the message will be reprocessed. Handlers must be designed to be safe to run twice on the same input (idempotent).

> Chapter 9 adds a dead-letter queue (DLQ) so persistent failures are routed to a separate topic for inspection rather than blocking or silently discarding messages.

### 202 Accepted vs 200 OK

| Code | Meaning |
|------|---------|
| `200 OK` | Request completed; body contains the result |
| `202 Accepted` | Request received and queued; outcome is not yet known |

`POST /trades/{id}/submit` returns 202 because the trade has been submitted to Kafka but the provider has not yet responded. Clients should poll `GET /trades/{id}` until the status is `FULFILLED` or `REJECTED`.

### Goroutines — quick recap

Every consumer runs in its own goroutine:

```go
go func() {
    defer c.Close()
    c.Run(ctx, handler)
}()
```

A goroutine is a lightweight thread managed by the Go runtime. Unlike Python threads, goroutines are multiplexed across OS threads by the scheduler — you can spawn thousands of them cheaply.

The `go` keyword is all you need. No thread pool configuration, no async/await.

### Graceful shutdown with `signal.NotifyContext`

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
```

`signal.NotifyContext` cancels `ctx` when the process receives SIGINT (Ctrl+C) or SIGTERM (Kubernetes stop signal). All consumers receive this ctx in their `Run` loop and exit cleanly when it is cancelled.

```go
// In KafkaConsumer.Run:
msg, err := c.reader.FetchMessage(ctx)
if err != nil {
    if ctx.Err() != nil {
        return nil // clean shutdown, not an error
    }
    return err
}
```

The HTTP server is given 10 seconds to drain in-flight requests before the process exits.

### `http.Server` vs `http.ListenAndServe`

Chapter 4 used the convenience function:

```go
http.ListenAndServe(":8080", mux)
```

Chapter 5 switches to an explicit `http.Server`:

```go
srv := &http.Server{Addr: *addr, Handler: mux}
go srv.ListenAndServe()
// ...
srv.Shutdown(shutdownCtx)
```

`http.Server.Shutdown` waits for active connections to complete before closing. `http.ListenAndServe` offers no such hook.

---

## Package structure

```
event/
  event.go          ← domain events (TradeSubmitted, TradeFulfilled, TradeRejected)
                      and topic name constants

broker/
  broker.go         ← Publisher and Consumer interfaces + Handler func type
  kafka.go          ← Kafka implementations (KafkaPublisher, KafkaConsumer)
  memory.go         ← InMemoryPublisher for testing

consumer/
  trade.go          ← TradeConsumer (HandleFulfilled, HandleRejected)
```

### Why separate `event/` and `broker/`?

`event/` is a **domain concept** — what things happened in the system. It has no knowledge of Kafka, HTTP, or any infrastructure.

`broker/` is an **infrastructure concern** — how messages are transported. The `Publisher` and `Consumer` interfaces mean you can swap Kafka for another broker (NATS, RabbitMQ, an in-memory fake) by changing the wiring in `main.go` only.

This separation is the same Ports & Adapters principle we applied to the store in Chapter 3.

### Why `consumer/` instead of adding to `handler/`?

`handler/` handles HTTP requests. `consumer/` handles Kafka messages. Both are adapters (in Hexagonal terms) but for different transports. Keeping them in separate packages makes it clear which type of event each function responds to.

---

## New files

### `event/event.go`

Three event types, each a plain struct with JSON tags:

```go
const (
    TopicTradeSubmitted = "trade-submitted"
    TopicTradeFulfilled = "trade-fulfilled"
    TopicTradeRejected  = "trade-rejected"
)

type TradeSubmitted struct {
    TradeID    string    `json:"tradeId"`
    Asset      string    `json:"asset"`
    Side       string    `json:"side"`
    Quantity   int       `json:"quantity"`
    LimitPrice float64   `json:"limitPrice"`
    OccurredAt time.Time `json:"occurredAt"`
}
```

### `broker/broker.go`

```go
type Publisher interface {
    Publish(ctx context.Context, topic string, v any) error
    Close() error
}

type Handler func(ctx context.Context, msg []byte) error

type Consumer interface {
    Run(ctx context.Context, handler Handler) error
    Close() error
}
```

`Handler` is a **function type** — a type whose values are functions. This is how Go does callbacks without heavyweight abstractions. You can pass any function with the matching signature:

```go
c.Run(ctx, tc.HandleFulfilled)  // method expression as a Handler
c.Run(ctx, func(ctx context.Context, msg []byte) error { ... }) // inline
```

### `broker/kafka.go` — KafkaPublisher

```go
func (p *KafkaPublisher) Publish(ctx context.Context, topic string, v any) error {
    data, _ := json.Marshal(v)
    return p.writer.WriteMessages(ctx, kafka.Message{
        Topic: topic,
        Value: data,
    })
}
```

One writer handles all topics. The `kafka.Writer` handles retries and batching internally.

### `broker/kafka.go` — KafkaConsumer

```go
func (c *KafkaConsumer) Run(ctx context.Context, handler Handler) error {
    for {
        msg, err := c.reader.FetchMessage(ctx)
        if err != nil { /* handle clean shutdown */ }

        if err := handler(ctx, msg.Value); err != nil {
            log.Printf("handler error (committing anyway): %v", err)
        }
        c.reader.CommitMessages(ctx, msg)
    }
}
```

`FetchMessage` blocks until a message arrives or `ctx` is cancelled. This is a standard Go pattern: instead of callbacks or async/await, the goroutine simply blocks and the Go scheduler switches to other work.

---

## Changes to existing files

### `handler/trade.go` — submit is now async

Before (Chapter 4):

```go
// Synchronous: blocks until the provider HTTP call completes.
resp, err := h.provider.Submit(r.Context(), req)
// ... transition to FULFILLED or REJECTED immediately
writeJSON(w, http.StatusOK, t)
```

After (Chapter 5):

```go
// Asynchronous: publish the event and return immediately.
h.publisher.Publish(r.Context(), event.TopicTradeSubmitted, evt)
writeJSON(w, http.StatusAccepted, t) // 202, status is SUBMITTED
```

The `provider.Provider` dependency is removed entirely. The handler now depends on `broker.Publisher` — a much smaller interface.

### `cmd/provider-stub/main.go` — no more HTTP server

The provider stub no longer exposes an HTTP endpoint. It is now a pure event consumer:

```go
c := broker.NewKafkaConsumer([]string{*brokers}, event.TopicTradeSubmitted, "provider-stub")
c.Run(ctx, h.handle)
```

Neither service needs to know the other's address. They only need to agree on topic names — the constants in `event/event.go`.

---

## Testing

### What the unit tests cover

| Package | Test | What it checks |
|---------|------|----------------|
| `broker` | `TestInMemoryPublisher_Publish` | Multiple topics, correct count |
| `broker` | `TestInMemoryPublisher_MessageContents` | Payload is valid JSON with correct fields |
| `consumer` | `TestHandleFulfilled` | Trade transitions SUBMITTED → FULFILLED |
| `consumer` | `TestHandleRejected` | Trade transitions SUBMITTED → REJECTED |
| `consumer` | `TestHandleFulfilled_UnknownTrade` | Returns error for unknown trade ID |
| `consumer` | `TestHandleFulfilled_InvalidJSON` | Returns unmarshal error on bad input |
| `handler` | `TestSubmitTrade/returns 202 Accepted` | Status code, body status, event count |
| `handler` | `TestSubmitTrade/already submitted` | 409 Conflict on double-submit |

### Testing asynchronous code synchronously

The tests exercise the full flow without a Kafka broker by using `broker.InMemoryPublisher`:

1. `POST /trades/{id}/submit` — handler publishes to the in-memory publisher
2. Assert `202 Accepted` and `SUBMITTED` status
3. Assert `pub.Count(event.TopicTradeSubmitted) == 1`
4. Unmarshal the captured message and inspect the payload

The consumer tests call `HandleFulfilled`/`HandleRejected` directly with hand-crafted JSON, skipping Kafka entirely. This makes tests fast and deterministic.

---

## Manual testing

### Prerequisites

- [Docker Desktop](https://www.docker.com/products/docker-desktop/) installed

### 1 — Start Kafka

```bash
docker compose up -d
```

Wait for the health check to pass:

```bash
docker compose ps
# kafka should show "healthy"
```

### 2 — Start both services (two terminals)

**Terminal 1 — trade API:**

```bash
go run ./cmd/trade-api
# trade-api listening on :8080
```

**Terminal 2 — provider stub:**

```bash
go run ./cmd/provider-stub
# provider stub listening for TradeSubmitted events...
```

### 3 — Create and submit a trade

```bash
# Create a trade
curl -s -X POST http://localhost:8080/trades \
  -H "Content-Type: application/json" \
  -d '{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}' | jq .
```

Copy the `id` from the response, then submit:

```bash
curl -s -X POST http://localhost:8080/trades/<id>/submit | jq .
# Response is 202 Accepted — status is "SUBMITTED"
```

### 4 — Watch the outcome arrive asynchronously

The trade is now SUBMITTED. A fraction of a second later the provider stub processes the event. Fetch the trade:

```bash
curl -s http://localhost:8080/trades/<id> | jq .status
# "FULFILLED"
```

Watch the provider stub terminal — you'll see:
```
evaluating order: tradeID=... asset=AAPL side=BUY qty=10 limit=195.00
```

### 5 — Test the rejection path

```bash
# limitPrice below 10.00 triggers a rejection
curl -s -X POST http://localhost:8080/trades \
  -H "Content-Type: application/json" \
  -d '{"asset":"GME","side":"BUY","quantity":100,"limitPrice":5.00}' | jq .id

curl -s -X POST http://localhost:8080/trades/<id>/submit | jq .

# Wait a moment, then:
curl -s http://localhost:8080/trades/<id> | jq .status
# "REJECTED"
```

### 6 — Observe Kafka topics (optional)

You can inspect the topics using the Kafka CLI inside the container:

```bash
# List topics
docker exec -it k8s-eda-kafka-1 kafka-topics.sh \
  --bootstrap-server localhost:9092 --list

# Tail the trade-submitted topic (Ctrl+C to stop)
docker exec -it k8s-eda-kafka-1 kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic trade-submitted \
  --from-beginning
```

You'll see the raw JSON payloads exactly as published by the trade API.

### 7 — Graceful shutdown

Press **Ctrl+C** in the trade-api terminal. The server drains in-flight requests, the consumers close cleanly, and the process exits without errors. Restart it and the consumers resume from where they left off — no events are lost.

### 8 — Stop Kafka

```bash
docker compose down
```

---

## Coming in Chapter 6

Chapter 6 introduces Go **generics** to build a `Store[T]` interface that can store any entity (trades, orders, portfolios) without duplicating the interface and error type for each. We also add an application service layer that sits between the HTTP handler and the store, and organise the code into an `internal/` package.