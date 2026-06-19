# Chapter 1 — Go Basics

**Branch:** `01-go-basics`

Before we touch HTTP, Kubernetes, or a message broker, we need a solid domain model to build on. This chapter introduces the Go concepts we'll use throughout the tutorial — structs, interfaces, enums, and error handling — grounded in the share trading domain.

By the end of this chapter you will have a working `Trade` model with lifecycle management, a persistence interface, and an in-memory implementation.

---

## What we're building

```
k8s-eda/
├── main.go          ← exercises the domain in a runnable program
└── trade/
    ├── trade.go     ← Trade struct, Side and Status enums, lifecycle transitions
    └── store.go     ← Store interface and InMemoryStore implementation
```

---

## Concepts covered

### Enums with `iota`

Go doesn't have a built-in `enum` keyword. The idiomatic approach is to define a named integer type and use `iota` to assign incrementing values to its constants.

```go
type Side int

const (
    Buy  Side = iota // 0
    Sell             // 1
)
```

`iota` resets to 0 at the start of each `const` block and increments by one for each constant. The named type (`Side`) means the compiler will reject you accidentally passing a plain `int` where a `Side` is expected.

We also implement `String()` on each enum type. Go's `fmt` package calls `String()` automatically whenever a value is printed, so `fmt.Println(Buy)` outputs `BUY` rather than `0`.

```go
func (s Side) String() string {
    switch s {
    case Buy:
        return "BUY"
    case Sell:
        return "SELL"
    default:
        return "UNKNOWN"
    }
}
```

### Structs

A struct groups related fields under a named type. In `trade.go` the `Trade` struct captures everything we need to know about a trading instruction:

```go
type Trade struct {
    ID         string
    Asset      string  // ticker symbol, e.g. "AAPL"
    Side       Side
    Quantity   int
    LimitPrice float64
    Status     Status
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

Fields are accessed with `.` notation: `t.Asset`, `t.Status`, and so on.

### Constructor functions and validation

Go doesn't have constructors in the class sense. The convention is a plain function — typically named `New` — that validates inputs and returns a pointer to the struct alongside an `error`.

```go
func New(asset string, side Side, quantity int, limitPrice float64) (*Trade, error) {
    if quantity <= 0 {
        return nil, fmt.Errorf("quantity must be positive, got %d", quantity)
    }
    // ...
    return &Trade{ /* fields */ }, nil
}
```

The `*Trade` return type is a pointer. When we return `&Trade{...}` we're returning the address of the value rather than a copy. This matters because callers will mutate the trade (updating its status), and we want those mutations to be visible everywhere the same trade is referenced.

Returning `nil, error` on failure is idiomatic Go. The caller is expected to check the error before using the value:

```go
t, err := trade.New("AAPL", trade.Buy, 10, 195.00)
if err != nil {
    // handle it
}
```

### Error handling

Go treats errors as ordinary values of the built-in `error` interface. There are no exceptions. Functions that can fail return an `error` as their last return value, and callers handle it explicitly.

For simple cases we use `fmt.Errorf`:

```go
return nil, fmt.Errorf("quantity must be positive, got %d", quantity)
```

For cases where the caller might need to inspect the error type (not just its message), we define a concrete error type:

```go
type ErrNotFound struct {
    ID string
}

func (e *ErrNotFound) Error() string {
    return fmt.Sprintf("trade not found: %s", e.ID)
}
```

The caller can then use `errors.As` to check for this specific type without string-matching the message:

```go
var notFound *trade.ErrNotFound
if errors.As(err, &notFound) {
    // we know the trade didn't exist — handle accordingly
}
```

This pattern becomes important in Chapter 2 when we need to return a `404` response for a missing trade.

### Interfaces

An interface defines a set of method signatures. Any type that implements all those methods satisfies the interface — there's no `implements` keyword.

```go
type Store interface {
    Save(t *Trade) error
    FindByID(id string) (*Trade, error)
    FindAll() ([]*Trade, error)
    FindByStatus(status Status) ([]*Trade, error)
}
```

`InMemoryStore` satisfies `Store` because it has all four methods with matching signatures. The compiler verifies this implicitly.

The value of the interface is that the rest of the code — `main.go`, and later our HTTP handler — only ever talks to `Store`. When we swap the in-memory implementation for a real database in Chapter 3, nothing else needs to change.

### Methods and receivers

A method is a function attached to a type via a *receiver*. We use pointer receivers (`*Trade`) when the method mutates the value:

```go
func (t *Trade) Transition(next Status) error {
    // ...
    t.Status = next  // mutates the trade
    t.UpdatedAt = time.Now()
    return nil
}
```

Using a value receiver (without `*`) would mutate a copy, leaving the original unchanged — a common source of bugs.

### State machine

`Transition` encodes the trade lifecycle as a simple state machine using a map from each status to its allowed successors:

```go
var validTransitions = map[Status][]Status{
    Pending:   {Submitted},
    Submitted: {Fulfilled, Rejected},
    Fulfilled: {},
    Rejected:  {},
}
```

Attempting an invalid move (e.g. `Fulfilled → Submitted`) returns an error rather than silently allowing it. Encoding the rules in data rather than a chain of `if` statements makes them easy to read and extend.

### Concurrency safety with `sync.RWMutex`

`InMemoryStore` wraps its map in a `sync.RWMutex`. This isn't needed in Chapter 1 (we run everything in a single goroutine) but it's a good habit — and from Chapter 2 onwards the HTTP server will handle requests concurrently.

```go
type InMemoryStore struct {
    mu     sync.RWMutex
    trades map[string]*Trade
}
```

`mu.Lock()` / `mu.Unlock()` for writes, `mu.RLock()` / `mu.RUnlock()` for reads (multiple reads can happen in parallel; writes are exclusive). `defer` ensures the lock is released even if the function panics.

---

## Running it

```bash
go run .
```

Expected output:

```
=== Placing trades ===
  ✓ Trade{ID: 3bfbd82b, Asset: AAPL, Side: BUY, Qty: 10, Limit: 195.00, Status: PENDING}
  ✓ Trade{ID: 27e07368, Asset: TSLA, Side: BUY, Qty: 5, Limit: 250.00, Status: PENDING}
  ✓ Trade{ID: 8e6fb13a, Asset: NVDA, Side: SELL, Qty: 3, Limit: 890.00, Status: PENDING}
  ✗ failed to create trade for MSFT: quantity must be positive, got 0

=== Advancing trade lifecycle ===
  ✓ AAPL → SUBMITTED
  ✓ AAPL → FULFILLED
  ✓ TSLA → SUBMITTED
  ✓ TSLA → REJECTED

=== Attempting an invalid transition ===
  ✗ transition blocked (expected): cannot transition trade from FULFILLED to SUBMITTED

=== Looking up a trade by ID ===
  found: Trade{ID: 3bfbd82b, Asset: AAPL, Side: BUY, Qty: 10, Limit: 195.00, Status: FULFILLED}

=== Looking up a non-existent trade ===
  ✗ not found (expected): trade not found: does-not-exist

=== All trades by status ===
  PENDING (1):
    • Trade{ID: 8e6fb13a, Asset: NVDA, Side: SELL, Qty: 3, Limit: 890.00, Status: PENDING}
  FULFILLED (1):
    • Trade{ID: 3bfbd82b, Asset: AAPL, Side: BUY, Qty: 10, Limit: 195.00, Status: FULFILLED}
  REJECTED (1):
    • Trade{ID: 27e07368, Asset: TSLA, Side: BUY, Qty: 5, Limit: 250.00, Status: REJECTED}
```

---

## What's next

In **Chapter 2** (`02-http-api`) we'll expose this domain over HTTP — a `POST /trades` endpoint to create a trade and a `GET /trades/:id` endpoint to retrieve one. The `Store` interface we defined here means the HTTP handler won't need to know anything about how trades are stored.
