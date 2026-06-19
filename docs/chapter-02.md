# Chapter 2 — HTTP API

**Branch:** `02-http-api`

In Chapter 1 we built the domain model as a self-contained Go program. In this chapter we put an HTTP server in front of it, exposing three endpoints:

| Method | Path | What it does |
|---|---|---|
| `POST` | `/trades` | Create a new trade |
| `GET` | `/trades/{id}` | Fetch a single trade by ID |
| `GET` | `/trades` | List all trades |

The domain model (`trade/trade.go`, `trade/store.go`) is **unchanged**. We're adding a delivery mechanism on top of it.

---

## What we're building

```
k8s-eda/
├── main.go              ← replaced: now starts an HTTP server
├── handler/
│   └── trade.go         ← new: HTTP handlers for the trade endpoints
└── trade/
    ├── trade.go         ← updated: JSON tags and marshaling added
    └── store.go         ← unchanged
```

---

## Concepts covered

### Go's standard library HTTP server

Go ships a production-capable HTTP server in `net/http` — no framework needed for this chapter. The two key types are:

- `http.ServeMux` — a router that maps URL patterns to handler functions
- `http.HandlerFunc` — any function with the signature `func(http.ResponseWriter, *http.Request)` can be registered as a handler

```go
mux := http.NewServeMux()
mux.HandleFunc("POST /trades", h.create)

log.Fatal(http.ListenAndServe(":8080", mux))
```

`log.Fatal` wraps `log.Println` followed by `os.Exit(1)`. Since `ListenAndServe` only returns if it encounters an error (e.g. the port is already in use), wrapping it in `log.Fatal` ensures we see the error and the program exits cleanly.

### Method-prefixed patterns and path parameters (Go 1.22+)

Before Go 1.22, the standard mux couldn't match by HTTP method or extract path segments — you'd reach for a third-party router. Go 1.22 added both:

```go
mux.HandleFunc("POST /trades", h.create)      // only matches POST
mux.HandleFunc("GET /trades/{id}", h.getByID) // {id} is a path parameter
mux.HandleFunc("GET /trades", h.list)         // only matches GET, no {id}
```

Inside `getByID`, the path parameter is retrieved with:

```go
id := r.PathValue("id")
```

### Dependency injection via constructor

`TradeHandler` doesn't create its own store — it receives one:

```go
type TradeHandler struct {
    store trade.Store  // interface, not a concrete type
}

func NewTradeHandler(store trade.Store) *TradeHandler {
    return &TradeHandler{store: store}
}
```

This is *dependency injection* — passing dependencies in rather than creating them internally. Because `store` is typed as the `Store` interface (not `*InMemoryStore`), the handler has no idea how trades are actually stored. In Chapter 3 we'll swap the in-memory store for a real one without touching a single line of handler code.

### JSON encoding and decoding

`encoding/json` is Go's built-in JSON package. For decoding a request body:

```go
var req createRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
    return
}
```

`NewDecoder` wraps the request body (an `io.Reader`) in a streaming JSON decoder. We pass `&req` — a pointer — so `Decode` can write into it.

For encoding a response:

```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusCreated)
json.NewEncoder(w).Encode(t)
```

`NewEncoder` wraps the `ResponseWriter` (also an `io.Writer`) and writes JSON directly to it.

### Struct tags

Struct tags tell `encoding/json` what key name to use for each field. Without them, Go uses the field name as-is (e.g. `LimitPrice`); with them you can control the exact JSON output:

```go
type Trade struct {
    ID         string    `json:"id"`
    LimitPrice float64   `json:"limitPrice"`
    CreatedAt  time.Time `json:"createdAt"`
    // ...
}
```

The tag is a raw string literal (backtick-quoted) containing key-value pairs. `json:"limitPrice"` means this field serialises as `"limitPrice"` in JSON.

### Custom JSON marshaling for enums

Our `Side` and `Status` types are `int` under the hood. Without custom marshaling they'd serialise as numbers (`0`, `1`), which isn't useful for an API consumer.

We implement `MarshalJSON` to control how they're written:

```go
func (s Side) MarshalJSON() ([]byte, error) {
    return json.Marshal(s.String()) // writes "BUY" or "SELL"
}
```

And `UnmarshalJSON` to parse them back from a string in the request body:

```go
func (s *Side) UnmarshalJSON(data []byte) error {
    var str string
    if err := json.Unmarshal(data, &str); err != nil {
        return err
    }
    switch str {
    case "BUY":
        *s = Buy
    case "SELL":
        *s = Sell
    default:
        return fmt.Errorf("unknown side %q, must be BUY or SELL", str)
    }
    return nil
}
```

Note `*s = Buy` — `s` is a pointer receiver (`*Side`), so we dereference it to assign the value. This is the same pointer pattern from Chapter 1.

### Mapping domain errors to HTTP status codes

The `ErrNotFound` type we defined in Chapter 1 now earns its keep. In the handler we use `errors.As` to check whether the error is specifically a "not found" error, and return a `404` accordingly:

```go
var notFound *trade.ErrNotFound
if errors.As(err, &notFound) {
    writeError(w, http.StatusNotFound, err.Error())
    return
}
// anything else is an unexpected internal error
writeError(w, http.StatusInternalServerError, "failed to retrieve trade")
```

This is why we defined a concrete error type rather than using a plain `fmt.Errorf` — it gives callers a way to branch on the error kind without string-matching the message.

---

## Running it

```bash
go run .
# server listening on :8080
```

Then in another terminal:

```bash
# Create a trade
curl -X POST http://localhost:8080/trades \
  -H 'Content-Type: application/json' \
  -d '{"asset":"AAPL","side":"BUY","quantity":10,"limitPrice":195.00}'

# List all trades
curl http://localhost:8080/trades

# Get a specific trade (replace <id> with an ID from above)
curl http://localhost:8080/trades/<id>

# Trigger a validation error
curl -X POST http://localhost:8080/trades \
  -H 'Content-Type: application/json' \
  -d '{"asset":"TSLA","side":"BUY","quantity":0,"limitPrice":250.00}'

# Trigger a 404
curl http://localhost:8080/trades/does-not-exist
```

---

## What's next

In **Chapter 3** (`03-persistence`) we'll replace the in-memory store with a real persistent store, introducing the repository pattern in more depth. Because the handler only depends on the `Store` interface, the swap will require zero changes to the handler code.
