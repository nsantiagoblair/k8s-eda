// Package store contains concrete implementations of trade.Store.
package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/nsantiagoblair/k8s-eda/trade"

	// The blank import registers the SQLite driver with database/sql.
	// We never reference modernc.org/sqlite directly — database/sql calls it
	// through the driver interface.
	_ "modernc.org/sqlite"
)

// SQLiteStore is a persistent implementation of trade.Store backed by SQLite.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at path and runs
// migrations to ensure the schema is up to date.
// Use ":memory:" as path for an in-process, ephemeral database (useful in tests).
func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &SQLiteStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// migrate creates the trades table if it doesn't already exist.
// In later chapters this would be handled by a migration tool (e.g. goose),
// but for now a single idempotent statement is enough.
func (s *SQLiteStore) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS trades (
			id          TEXT PRIMARY KEY,
			asset       TEXT    NOT NULL,
			side        TEXT    NOT NULL,
			quantity    INTEGER NOT NULL,
			limit_price REAL    NOT NULL,
			status      TEXT    NOT NULL,
			created_at  TEXT    NOT NULL,
			updated_at  TEXT    NOT NULL
		)
	`)
	return err
}

// Close releases the underlying database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// Save inserts a new trade or replaces an existing one with the same ID.
func (s *SQLiteStore) Save(t *trade.Trade) error {
	_, err := s.db.Exec(`
		INSERT INTO trades (id, asset, side, quantity, limit_price, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status     = excluded.status,
			updated_at = excluded.updated_at
	`,
		t.ID,
		t.Asset,
		t.Side.String(),
		t.Quantity,
		t.LimitPrice,
		t.Status.String(),
		t.CreatedAt.Format(time.RFC3339Nano),
		t.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("save trade %s: %w", t.ID, err)
	}
	return nil
}

// FindByID retrieves a single trade by its ID, returning ErrNotFound if it
// doesn't exist.
func (s *SQLiteStore) FindByID(id string) (*trade.Trade, error) {
	row := s.db.QueryRow(`
		SELECT id, asset, side, quantity, limit_price, status, created_at, updated_at
		FROM trades WHERE id = ?
	`, id)

	t, err := scanTrade(row.Scan)
	if err == sql.ErrNoRows {
		return nil, &trade.ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find trade %s: %w", id, err)
	}
	return t, nil
}

// FindAll returns every trade in the store.
func (s *SQLiteStore) FindAll() ([]*trade.Trade, error) {
	rows, err := s.db.Query(`
		SELECT id, asset, side, quantity, limit_price, status, created_at, updated_at
		FROM trades ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("find all trades: %w", err)
	}
	defer rows.Close()

	return collectRows(rows)
}

// FindByStatus returns all trades with the given status.
func (s *SQLiteStore) FindByStatus(status trade.Status) ([]*trade.Trade, error) {
	rows, err := s.db.Query(`
		SELECT id, asset, side, quantity, limit_price, status, created_at, updated_at
		FROM trades WHERE status = ? ORDER BY created_at ASC
	`, status.String())
	if err != nil {
		return nil, fmt.Errorf("find trades by status %s: %w", status, err)
	}
	defer rows.Close()

	return collectRows(rows)
}

// --- helpers ----------------------------------------------------------------

// scanFunc is the common signature for both row.Scan and rows.Scan, letting
// us share the scanning logic between single-row and multi-row queries.
type scanFunc func(dest ...any) error

func scanTrade(scan scanFunc) (*trade.Trade, error) {
	var (
		t                        trade.Trade
		sideStr, statusStr       string
		createdAtStr, updatedAt  string
	)

	err := scan(
		&t.ID, &t.Asset, &sideStr, &t.Quantity, &t.LimitPrice,
		&statusStr, &createdAtStr, &updatedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Side, err = trade.ParseSide(sideStr)
	if err != nil {
		return nil, err
	}
	t.Status, err = trade.ParseStatus(statusStr)
	if err != nil {
		return nil, err
	}
	t.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("parse created_at: %w", err)
	}
	t.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("parse updated_at: %w", err)
	}

	return &t, nil
}

func collectRows(rows *sql.Rows) ([]*trade.Trade, error) {
	var trades []*trade.Trade
	for rows.Next() {
		t, err := scanTrade(rows.Scan)
		if err != nil {
			return nil, err
		}
		trades = append(trades, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if trades == nil {
		trades = []*trade.Trade{}
	}
	return trades, nil
}
