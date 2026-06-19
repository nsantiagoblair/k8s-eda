# Chapter 3 — Persistence

**Branch:** `03-persistence`

In Chapter 2 trades were held in memory — restart the server and they're gone. In this chapter we replace the `InMemoryStore` with a SQLite-backed store that persists data to disk.

The headline result: **the handler code is completely unchanged.** This is the `Store` interface from Chapter 1 paying off in practice.

---

## What we're building

```
k8s-eda/
├── main.go              ← updated: wires SQLiteStore instead of InMemoryStore
├── store/
│   └── sqlite.go        ← new: SQLiteStore implementing trade.Store
├── handler/
│   └── trade.go         ← unchanged
└── trade/
    ├── trade.go         ← updated: ParseSide and ParseStatus helpers added
    └── store.go         ← unchanged
```

---

## Concepts covered

### `database/sql` — Go's database interface

Go's standard library provides `database/sql`: a generic interface for relational databases. It defines types like `*sql.DB`, `*sql.Rows`, and `*sql.Row` that work identically regardless of which database you're using.

The actual database-specific behaviour lives in a *driver* — a separate package that registers itself with `database/sql` on import. We use the pure-Go SQLite driver:

```go
import _ "modernc.org/sqlite"
```

The blank import (`_`) means "import this package for its side effects" — in this case, registering the driver. We never reference `modernc.org/sqlite` directly in our code. If we wanted to switch to PostgreSQL later, we'd swap this one import for a Postgres driver, and nothing else would change.

### Opening a database

```go
db, err := sql.Open("sqlite", path)
```

`sql.Open` doesn't actually connect to the database — it just validates the driver name and the data source string. The connection is made lazily on first use. Use `":memory:"` as the path for an in-process database that lives only for the lifetime of the program (very useful in tests).

### Migrations

Before we can store trades we need the `trades` table to exist. We run a migration on startup:

```go
func (s *SQLiteStore) migrate() error {
    _, err := s.db.Exec(`
        CREATE TABLE IF NOT EXISTS trades (
            id          TEXT PRIMARY KEY,
            ...
        )
    `)
    return err
}
```

`CREATE TABLE IF NOT EXISTS` is idempotent — it's safe to run every time the server starts. In later chapters (once we have a real Postgres database) we'd use a migration tool like [goose](https://github.com/pressly/goose) to manage schema changes properly.

### Parameterised queries

Never build SQL strings by concatenating user input — that's how SQL injection happens. Always use `?` placeholders:

```go
s.db.Exec(`
    INSERT INTO trades (...) VALUES (?, ?, ?, ...)
`, t.ID, t.Asset, t.Side.String(), ...)
```

`database/sql` substitutes the values safely, escaping them correctly for the database driver.

### Upsert — INSERT OR UPDATE

When `Save` is called on an existing trade (e.g. after a status transition), we want to update rather than fail with a duplicate key error. SQLite's `ON CONFLICT` clause handles this:

```go
INSERT INTO trades (...) VALUES (?, ...)
ON CONFLICT(id) DO UPDATE SET
    status     = excluded.status,
    updated_at = excluded.updated_at
```

`excluded` refers to the row that was about to be inserted. This is an *upsert* — insert if new, update specific fields if the ID already exists.

### Scanning rows

Reading from the database requires *scanning* each column into a Go variable:

```go
row := s.db.QueryRow(`SELECT ... FROM trades WHERE id = ?`, id)

var t trade.Trade
var sideStr, statusStr string
err := row.Scan(&t.ID, &t.Asset, &sideStr, ...)
```

We store `Side` and `Status` as their string representations (`"BUY"`, `"PENDING"` etc.) rather than integers. This makes the database human-readable and means the data still makes sense if you query it directly. We added `ParseSide` and `ParseStatus` to the `trade` package to convert them back.

### Sharing scan logic

Both `FindByID` (one row) and `FindAll`/`FindByStatus` (multiple rows) need to scan the same columns. Rather than duplicating the code, we define a `scanFunc` type and a `scanTrade` helper that both call:

```go
type scanFunc func(dest ...any) error

func scanTrade(scan scanFunc) (*trade.Trade, error) { ... }

// Used for single row:
t, err := scanTrade(row.Scan)

// Used for multiple rows:
for rows.Next() {
    t, err := scanTrade(rows.Scan)
}
```

`row.Scan` and `rows.Scan` have the same signature, so they both satisfy `scanFunc`.

### `rows.Close()` and `defer`

When querying multiple rows, you must close the result set when done — it holds a database connection until closed:

```go
rows, err := s.db.Query(...)
defer rows.Close() // always runs, even if we return early
```

Forgetting `rows.Close()` is a common source of connection leaks in Go database code.

### The interface paying off

The only change to `main.go` is swapping one line:

```go
// Before (Chapter 2)
store := trade.NewInMemoryStore()

// After (Chapter 3)
s, err := store.NewSQLiteStore("trades.db")
```

`handler.NewTradeHandler` still receives a `trade.Store`. It doesn't know or care that trades are now written to disk. This is the point of the interface — we changed the entire persistence mechanism without touching the delivery layer.

> **Coming in Chapter 6:** Right now `Store` is tied to `Trade` specifically — every method signature mentions `*Trade`. When we add `Order` and `Portfolio` entities we'd need to write near-identical interfaces and implementations for each. That's when we'll introduce Go generics: a single `Store[T any]` interface that both `InMemoryStore` and `SQLiteStore` can implement for any entity type.

---

## Testing

SQLite has a killer feature for testing: `:memory:` databases. Each test gets its own isolated, in-memory database with no files to clean up:

```go
func newTestStore(t *testing.T) *store.SQLiteStore {
    s, _ := store.NewSQLiteStore(":memory:")
    t.Cleanup(func() { s.Close() })
    return s
}
```

`t.Cleanup` registers a function to run when the test finishes — the Go equivalent of a teardown. It runs even if the test fails.

Notice that the SQLite tests (`store/sqlite_test.go`) cover exactly the same behaviours as the in-memory tests (`trade/store_test.go`): save, find, not-found, filter by status. Both stores implement the same interface, so they should pass the same contract. This is the basis of *contract testing* — a pattern we'll explore further when we have more implementations.

### Running the tests

```bash
go test ./...
```

---

## Running it

```bash
go run .
# server listening on :8080
```

Trades now survive a server restart. The `trades.db` file is created in the working directory (and is `.gitignore`d so it doesn't end up in source control).

---

## What's next

In **Chapter 4** (`04-provider-stub`) we'll introduce a second service: a stub trading provider that accepts orders and returns fulfilments. This sets up the asynchronous workflow that the event-driven architecture in Chapter 5 will replace.
