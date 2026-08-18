// Package sqlite implements sync-state persistence with modernc.org/sqlite.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/gloamers/openproject-bridge/internal/defaults"
	"github.com/gloamers/openproject-bridge/internal/storage"

	_ "modernc.org/sqlite"
)

// Store is the SQLite driver for deliveries and issue mappings.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database at path.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", fmt.Sprintf("%s?_pragma=busy_timeout(%d)", path, defaults.SQLiteBusyMs))
	if err != nil {
		return nil, fmt.Errorf("storage/sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage/sqlite: wal: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS deliveries (
  delivery_id TEXT PRIMARY KEY,
  status TEXT NOT NULL DEFAULT 'done',
  processed_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS issues (
  owner TEXT NOT NULL,
  repo TEXT NOT NULL,
  number INTEGER NOT NULL,
  org_id TEXT NOT NULL,
  project_id INTEGER NOT NULL,
  wp_id INTEGER NOT NULL,
  wp_url TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL,
  PRIMARY KEY (owner, repo, number)
);
CREATE INDEX IF NOT EXISTS idx_issues_wp ON issues(wp_id);
`)
	if err != nil {
		return err
	}
	// Older DBs may lack status column.
	_, _ = s.db.Exec(`ALTER TABLE deliveries ADD COLUMN status TEXT NOT NULL DEFAULT 'done'`)
	return nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// BeginDelivery marks a delivery as in-flight.
// started=true means this caller owns processing.
// alreadyDone=true means a prior run completed successfully (idempotent skip).
func (s *Store) BeginDelivery(ctx context.Context, deliveryID string) (started, alreadyDone bool, err error) {
	if deliveryID == "" {
		return true, false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO deliveries(delivery_id, status, processed_at) VALUES(?, 'pending', ?)`,
		deliveryID, now,
	)
	if err != nil {
		return false, false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, false, err
	}
	if n == 1 {
		return true, false, nil
	}
	var status string
	err = s.db.QueryRowContext(ctx, `SELECT status FROM deliveries WHERE delivery_id = ?`, deliveryID).Scan(&status)
	if err != nil {
		return false, false, err
	}
	if status == "done" {
		return false, true, nil
	}
	return false, false, nil
}

// CompleteDelivery marks a delivery as successfully processed.
func (s *Store) CompleteDelivery(ctx context.Context, deliveryID string) error {
	if deliveryID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE deliveries SET status='done', processed_at=? WHERE delivery_id=?`,
		time.Now().UTC().Format(time.RFC3339Nano), deliveryID,
	)
	return err
}

// FailDelivery removes a pending claim so GitHub redelivery can retry.
func (s *Store) FailDelivery(ctx context.Context, deliveryID string) error {
	if deliveryID == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM deliveries WHERE delivery_id=? AND status='pending'`, deliveryID,
	)
	return err
}

// UpsertIssue stores or updates an issue mapping.
func (s *Store) UpsertIssue(ctx context.Context, m storage.IssueMapping) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO issues(owner, repo, number, org_id, project_id, wp_id, wp_url, updated_at)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(owner, repo, number) DO UPDATE SET
  org_id=excluded.org_id,
  project_id=excluded.project_id,
  wp_id=excluded.wp_id,
  wp_url=excluded.wp_url,
  updated_at=excluded.updated_at
`, m.Owner, m.Repo, m.Number, m.OrgID, m.ProjectID, m.WPID, m.WPURL, time.Now().UTC().Format(time.RFC3339Nano))
	return err
}

// GetIssue loads a mapping by GitHub issue key.
func (s *Store) GetIssue(ctx context.Context, k storage.IssueKey) (*storage.IssueMapping, error) {
	var m storage.IssueMapping
	err := s.db.QueryRowContext(ctx, `
SELECT owner, repo, number, org_id, project_id, wp_id, wp_url
FROM issues WHERE owner=? AND repo=? AND number=?
`, k.Owner, k.Repo, k.Number).Scan(&m.Owner, &m.Repo, &m.Number, &m.OrgID, &m.ProjectID, &m.WPID, &m.WPURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// ListIssues returns all mappings (for reconcile).
func (s *Store) ListIssues(ctx context.Context) ([]storage.IssueMapping, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT owner, repo, number, org_id, project_id, wp_id, wp_url FROM issues`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storage.IssueMapping
	for rows.Next() {
		var m storage.IssueMapping
		if err := rows.Scan(&m.Owner, &m.Repo, &m.Number, &m.OrgID, &m.ProjectID, &m.WPID, &m.WPURL); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
