package store

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/nsantiagoblair/k8s-eda/internal/trade"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a persistent implementation of Store[*trade.Trade] backed by SQLite.
// SQL schemas are entity-specific so this store is not generic — unlike InMemoryStore[T].
// The trade-agnostic Store[T] interface is still satisfied, allowing the service layer
// to swap between SQLiteStore and InMemoryStore[*trade.Trade] without changes.
type SQLiteStore struct {
	db *sql.DB
}

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

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) Save(t *trade.Trade) error {
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
	if err != nil {
		return fmt.Errorf("save trade %s: %w", t.ID, err)
	}
	return nil
}

func (s *SQLiteStore) FindByID(id string) (*trade.Trade, error) {
	row := s.db.QueryRow(`
		SELECT id, asset, side, quantity, limit_price, status, created_at, updated_at
		FROM trades WHERE id = ?
	`, id)

	t, err := scanTrade(row.Scan)
	if err == sql.ErrNoRows {
		return nil, &ErrNotFound{ID: id}
	}
	if err != nil {
		return nil, fmt.Errorf("find trade %s: %w", id, err)
	}
	return t, nil
}

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

// FindByStatus returns all trades with the given status. This method is not
// part of Store[T] — it is a trade-specific query available on the concrete
// type when callers need SQL-level filtering.
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

type scanFunc func(dest ...any) error

func scanTrade(scan scanFunc) (*trade.Trade, error) {
	var (
		t                       trade.Trade
		sideStr, statusStr      string
		createdAtStr, updatedAt string
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
