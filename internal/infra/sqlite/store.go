// Package sqlite provides a SQLite-backed implementation of store.Store[*trade.Trade].
// It lives under infra/ because it depends on an external system (the SQLite engine)
// and would be swapped out for a Postgres implementation in production (Chapter 9).
package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/nsantiagoblair/k8s-eda/internal/store"
	"github.com/nsantiagoblair/k8s-eda/internal/trade"

	_ "modernc.org/sqlite"
)

// Store is a persistent trade store backed by SQLite.
// The type name is just Store — the package name (sqlite) provides the context:
// callers reference it as sqlite.Store, sqlite.NewStore.
type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
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

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Save(t *trade.Trade) error {
	_, err := s.db.Exec(`
		INSERT INTO trades (id, asset, side, quantity, limit_price, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status     = excluded.status,
			updated_at = excluded.updated_at
	`,
		t.ID, t.Asset, t.Side.String(), t.Quantity, t.LimitPrice,
		t.Status.String(),
		t.CreatedAt.Format(time.RFC3339Nano),
		t.UpdatedAt.Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) FindByID(id string) (*trade.Trade, error) {
	row := s.db.QueryRow(`
		SELECT id, asset, side, quantity, limit_price, status, created_at, updated_at
		FROM trades WHERE id = ?
	`, id)
	t, err := scanTrade(row.Scan)
	if err == sql.ErrNoRows {
		return nil, &store.ErrNotFound{ID: id}
	}
	return t, err
}

func (s *Store) FindAll() ([]*trade.Trade, error) {
	rows, err := s.db.Query(`
		SELECT id, asset, side, quantity, limit_price, status, created_at, updated_at
		FROM trades ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectRows(rows)
}

// --- helpers ----------------------------------------------------------------

type scanFunc func(dest ...any) error

func scanTrade(scan scanFunc) (*trade.Trade, error) {
	var (
		t                       trade.Trade
		sideStr, statusStr      string
		createdAtStr, updatedAt string
	)
	err := scan(&t.ID, &t.Asset, &sideStr, &t.Quantity, &t.LimitPrice,
		&statusStr, &createdAtStr, &updatedAt)
	if err != nil {
		return nil, err
	}
	if t.Side, err = trade.ParseSide(sideStr); err != nil {
		return nil, err
	}
	if t.Status, err = trade.ParseStatus(statusStr); err != nil {
		return nil, err
	}
	if t.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAtStr); err != nil {
		return nil, err
	}
	if t.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt); err != nil {
		return nil, err
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
