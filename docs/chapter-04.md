# Chapter 4 — Provider Stub

**Branch:** `04-provider-stub`

Trades currently sit in `PENDING` forever — nothing moves them forward. In this chapter we introduce a second service: a **provider stub** that simulates an external trade execution system. The trade API submits trades to the provider over HTTP and transitions them to `FULFILLED` or `REJECTED` based on the response.

This is still a synchronous, request/response flow. In Chapter 5 we'll replace it with event-driven communication — but building it synchronously first makes the problem concrete and the motivation for events clear.

---

## What we're building

```
k8s-eda/
├── cmd/
│   ├── trade-api/
│   │   └── main.go          ← moved from root; now accepts --provider-url flag
│   └── provider-stub/
│       └── main.go          ← new: stub HTTP server on :9090
├── provider/
│   ├── provider.go          ← new: Provider interface + SubmitRequest/Response types
│   └── client.go            ← new: HTTP client implementing Provider
├── handler/
│   └── trade.go             ← updated: Provider injected; POST /trades/{id}/submit added
└── trade/, store/           ← unchanged
```

The root `main.go` is removed — entry points now live under `cmd/`.

---

## Concepts covered

### The `cmd/` layout

When a project has more than one binary, the Go convention is to put each entry point under `cmd/<name>/main.go`:

```
cmd/
  trade-api/main.go       ← go run ./cmd/trade-api
  provider-stub/main.go   ← go run ./cmd/provider-stub
```

Each is still `package main`. The module path doesn't change — they both import from `github.com/nsantiagoblair/k8s-eda/...`. Only their location in the directory tree changes.

### CLI flags with `flag`

Rather than hardcoding the provider URL and listen address, `cmd/trade-api/main.go` reads them from flags:

```go
addr        := flag.String("addr",         ":8080",                "address to listen on")
providerURL := flag.String("provider-url", "http://localhost:9090", "provider stub base URL")
dbPath      := flag.String("db",           "trades.db",            "path to SQLite database")
flag.Parse()
```

`flag.String` declares a flag and returns a pointer to its value. `flag.Parse()` reads `os.Args` and fills in the values. The defaults mean `go run ./cmd/trade-api` just works; flags are only needed to override them.

### The `Provider` interface

We define a `Provider` interface in the `provider` package, following the same pattern as `trade.Store`:

```go
type Provider interface {
    Submit(ctx context.Context, req SubmitRequest) (*SubmitResponse, error)
}
```

The handler accepts `provider.Provider` — not `*provider.Client`. This means tests can inject a `mockProvider` without starting a real HTTP server, and a future real provider integration can be swapped in without touching the handler.

### Making HTTP requests

The standard library `net/http` package handles both serving and making HTTP requests. The key types on the client side:

- `http.Client` — manages connections; set `Timeout` to avoid hanging indefinitely
- `http.NewRequestWithContext` — builds a request tied to a context
- `resp.Body` — must be closed after reading, or the connection leaks

```go
httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
httpReq.Header.Set("Content-Type", "application/json")
resp, err := c.httpClient.Do(httpReq)
defer resp.Body.Close()
```

### `context.Context`

`context.Context` carries a cancellation signal and optional deadline through a call chain. We pass `r.Context()` — the HTTP request's context — through to the provider call:

```go
resp, err := h.provider.Submit(r.Context(), req)
```

`r.Context()` is automatically cancelled when the client disconnects. By threading it through, the outbound HTTP request to the provider is also cancelled — Go's HTTP client will abort the in-flight request and the provider stub's handler will unblock. Without this, the provider call would continue running even after the client has gone away, wasting resources.

This is the standard Go pattern: always pass `context.Context` as the first argument to any function that does I/O.

### The submit endpoint

`POST /trades/{id}/submit` orchestrates the lifecycle transition:

```
1. Find trade (→ 404 if missing)
2. Transition PENDING → SUBMITTED (→ 409 Conflict if already submitted/fulfilled/rejected)
3. Save (SUBMITTED state persisted before calling provider)
4. Call provider
5. On success: transition → FULFILLED or REJECTED, save
6. On provider error: return 502, trade stays SUBMITTED
```

Saving the `SUBMITTED` state before calling the provider is intentional. If the server crashes mid-call, the trade isn't silently stuck in `PENDING` — it's `SUBMITTED`, which signals it was in-flight. In Chapter 9 (Resilience) we'll handle recovery from this state.

### Error status codes

| Situation | HTTP status |
|---|---|
| Trade not found | `404 Not Found` |
| Trade not in PENDING state | `409 Conflict` |
| Provider call failed | `502 Bad Gateway` |
| Unexpected internal error | `500 Internal Server Error` |

`502 Bad Gateway` is the correct code when *your* service is acting as a gateway and the *upstream* service fails. Using `500` would be misleading — the problem isn't in the trade API, it's in the provider.

### Testing with mocks

Because `Provider` is an interface, tests never need to start the provider stub. A `mockProvider` struct is defined directly in the test file:

```go
type mockProvider struct {
    response *provider.SubmitResponse
    err      error
}

func (m *mockProvider) Submit(_ context.Context, _ provider.SubmitRequest) (*provider.SubmitResponse, error) {
    return m.response, m.err
}
```

Tests then inject whichever variant they need: `fulfilledProvider()`, `rejectedProvider()`, or `unavailableProvider()`. This is the interface pattern from Chapter 1 applied to testability — the same reason `Store` is an interface.

For the `Client` itself, `provider/client_test.go` uses `httptest.NewServer` to stand up a real HTTP server in-process, exercising the full serialisation/deserialisation cycle without needing the actual provider stub running.

---

## Manual testing

### 1. Start both services

Open two terminals:

```bash
# Terminal 1 — provider stub
go run ./cmd/provider-stub
# provider stub listening on :9090

# Terminal 2 — trade API
go run ./cmd/trade-api
# trade-api listening on :8080 (provider: http://localhost:9090)
```

### 2. Create a trade

```bash
curl -s -X POST http://localhost:8080/trades \
  -H 'Content-Type: application/json' \
  -d '{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}' | jq .
```

Copy the `id` from the response.

### 3. Submit the trade

```bash
curl -s -X POST http://localhost:8080/trades/<id>/submit | jq .
```

The response should show `"status": "FULFILLED"`. Watch the provider stub terminal — you'll see a log line confirming it received the order.

### 4. Trigger a rejection

Create a trade with a very low limit price:

```bash
curl -s -X POST http://localhost:8080/trades \
  -H 'Content-Type: application/json' \
  -d '{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":5.00}' | jq .

curl -s -X POST http://localhost:8080/trades/<id>/submit | jq .
# "status": "REJECTED"
```

### 5. Try to submit a fulfilled trade (conflict)

```bash
# Submit a trade that's already FULFILLED
curl -s -X POST http://localhost:8080/trades/<fulfilled-id>/submit | jq .
# { "error": "cannot transition trade from FULFILLED to SUBMITTED" }
# HTTP 409 Conflict
```

### 6. Simulate provider outage

Stop the provider stub (`Ctrl+C` in Terminal 1) and try to submit a new trade:

```bash
curl -s -X POST http://localhost:8080/trades/<new-id>/submit | jq .
# { "error": "provider unavailable: ..." }
# HTTP 502 Bad Gateway
```

The trade is now `SUBMITTED` — it was sent but unconfirmed. Restart the provider stub and you'll see this is exactly the problem Chapter 5 solves with events.

---

## What's next

In **Chapter 5** (`05-events`) we'll replace this synchronous HTTP call with event-driven communication. Instead of the trade API waiting for the provider's response, it will publish a `TradeSubmitted` event and move on. The provider stub will consume the event, process the order, and publish a `TradeFulfilled` or `TradeRejected` event back — which the trade API will consume to update the trade's status.

This removes the tight coupling between the two services and means neither has to know the other's URL.
