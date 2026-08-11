// Package store persists bills and model votes in SQLite so that OpenRouter is
// only billed once per (bill, model) pair.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// ErrNotFound is returned when a bill does not exist.
var ErrNotFound = errors.New("not found")

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS bills (
    id              TEXT PRIMARY KEY,
    number          TEXT NOT NULL,
    title           TEXT NOT NULL,
    chamber         TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT '',
    status_category TEXT NOT NULL DEFAULT '',
    policy_area     TEXT NOT NULL DEFAULT '',
    sponsor         TEXT NOT NULL DEFAULT '',
    sponsor_party   TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    full_text       TEXT NOT NULL DEFAULT '',
    ideology_score  REAL NOT NULL DEFAULT 0,
    source          TEXT NOT NULL DEFAULT 'seed',
    source_url      TEXT NOT NULL DEFAULT '',
    introduced_date INTEGER NOT NULL DEFAULT 0,
    updated_at      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS votes (
    bill_id    TEXT NOT NULL,
    model_key  TEXT NOT NULL,
    vote       TEXT NOT NULL,
    reason     TEXT NOT NULL DEFAULT '',
    error      TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (bill_id, model_key),
    FOREIGN KEY (bill_id) REFERENCES bills(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bills_updated ON bills(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_votes_model ON votes(model_key);
`

// Open creates (or opens) the database at path and applies the schema.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	// modernc's driver serialises fine, but a single writer avoids SQLITE_BUSY
	// churn when the vote workers all finish at once.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// CountBills returns the number of stored bills.
func (s *Store) CountBills(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM bills`).Scan(&n)
	return n, err
}

// UpsertBill inserts or updates a bill, preserving a previously computed
// ideology score when the incoming bill does not carry one.
func (s *Store) UpsertBill(ctx context.Context, b models.Bill) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO bills (id, number, title, chamber, status, status_category, policy_area, sponsor,
                   sponsor_party, summary, full_text, ideology_score, source, source_url,
                   introduced_date, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
    number=excluded.number,
    title=excluded.title,
    chamber=excluded.chamber,
    status=excluded.status,
    status_category=excluded.status_category,
    policy_area=excluded.policy_area,
    sponsor=excluded.sponsor,
    sponsor_party=excluded.sponsor_party,
    summary=excluded.summary,
    full_text=excluded.full_text,
    ideology_score=CASE WHEN excluded.ideology_score = 0 THEN bills.ideology_score ELSE excluded.ideology_score END,
    source=excluded.source,
    source_url=excluded.source_url,
    introduced_date=excluded.introduced_date,
    updated_at=excluded.updated_at`,
		b.ID, b.Number, b.Title, b.Chamber, b.Status, b.StatusCategory, b.PolicyArea, b.Sponsor,
		b.SponsorParty, b.Summary, b.FullText, b.IdeologyScore, b.Source, b.SourceURL,
		unix(b.IntroducedDate), unix(b.UpdatedAt))
	return err
}

// SetIdeologyScore stores a computed ideology score for a bill.
func (s *Store) SetIdeologyScore(ctx context.Context, billID string, score float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bills SET ideology_score = ? WHERE id = ?`, score, billID)
	return err
}

const billColumns = `id, number, title, chamber, status, status_category, policy_area, sponsor,
	sponsor_party, summary, full_text, ideology_score, source, source_url, introduced_date, updated_at`

func scanBill(rows interface{ Scan(...any) error }) (models.Bill, error) {
	var b models.Bill
	var introduced, updated int64
	err := rows.Scan(&b.ID, &b.Number, &b.Title, &b.Chamber, &b.Status, &b.StatusCategory,
		&b.PolicyArea, &b.Sponsor, &b.SponsorParty, &b.Summary, &b.FullText, &b.IdeologyScore,
		&b.Source, &b.SourceURL, &introduced, &updated)
	if err != nil {
		return models.Bill{}, err
	}
	b.IntroducedDate = fromUnix(introduced)
	b.UpdatedAt = fromUnix(updated)
	return b, nil
}

// Bill loads a single bill by id.
func (s *Store) Bill(ctx context.Context, id string) (models.Bill, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+billColumns+` FROM bills WHERE id = ?`, id)
	b, err := scanBill(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bill{}, ErrNotFound
	}
	return b, err
}

// Bills returns every bill, newest first.
func (s *Store) Bills(ctx context.Context) ([]models.Bill, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+billColumns+` FROM bills ORDER BY updated_at DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Bill
	for rows.Next() {
		b, err := scanBill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// LatestBill returns the most recently updated bill.
func (s *Store) LatestBill(ctx context.Context) (models.Bill, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+billColumns+` FROM bills ORDER BY updated_at DESC, id ASC LIMIT 1`)
	b, err := scanBill(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Bill{}, ErrNotFound
	}
	return b, err
}

// SaveVote records one model's verdict.
func (s *Store) SaveVote(ctx context.Context, v models.Vote) error {
	if v.CreatedAt.IsZero() {
		v.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO votes (bill_id, model_key, vote, reason, error, created_at)
VALUES (?,?,?,?,?,?)
ON CONFLICT(bill_id, model_key) DO UPDATE SET
    vote=excluded.vote, reason=excluded.reason, error=excluded.error, created_at=excluded.created_at`,
		v.BillID, v.ModelKey, v.Vote, v.Reason, v.Error, unix(v.CreatedAt))
	return err
}

// DeleteVotes clears all recorded verdicts for a bill.
func (s *Store) DeleteVotes(ctx context.Context, billID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM votes WHERE bill_id = ?`, billID)
	return err
}

// VotesByBill returns stored verdicts keyed by bill id then model key.
func (s *Store) VotesByBill(ctx context.Context, billIDs ...string) (map[string]map[string]models.Vote, error) {
	query := `SELECT bill_id, model_key, vote, reason, error, created_at FROM votes`
	args := make([]any, 0, len(billIDs))
	if len(billIDs) > 0 {
		query += ` WHERE bill_id IN (` + placeholders(len(billIDs)) + `)`
		for _, id := range billIDs {
			args = append(args, id)
		}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]map[string]models.Vote{}
	for rows.Next() {
		var v models.Vote
		var created int64
		if err := rows.Scan(&v.BillID, &v.ModelKey, &v.Vote, &v.Reason, &v.Error, &created); err != nil {
			return nil, err
		}
		v.CreatedAt = fromUnix(created)
		if m, ok := models.ModelByKey(v.ModelKey); ok {
			v.ModelName = m.Name
		}
		if out[v.BillID] == nil {
			out[v.BillID] = map[string]models.Vote{}
		}
		out[v.BillID][v.ModelKey] = v
	}
	return out, rows.Err()
}

// WithVotes decorates bills with their verdicts, inserting Pending placeholders
// for models that have not answered yet so the UI always renders every column.
func (s *Store) WithVotes(ctx context.Context, bills []models.Bill) ([]models.BillWithVotes, error) {
	ids := make([]string, 0, len(bills))
	for _, b := range bills {
		ids = append(ids, b.ID)
	}
	byBill := map[string]map[string]models.Vote{}
	if len(ids) > 0 {
		var err error
		byBill, err = s.VotesByBill(ctx, ids...)
		if err != nil {
			return nil, err
		}
	}

	out := make([]models.BillWithVotes, 0, len(bills))
	for _, b := range bills {
		bw := models.BillWithVotes{Bill: b, Votes: make([]models.Vote, 0, len(models.Catalog))}
		for _, m := range models.Catalog {
			if v, ok := byBill[b.ID][m.Key]; ok {
				bw.Votes = append(bw.Votes, v)
				continue
			}
			bw.Votes = append(bw.Votes, models.Vote{
				BillID: b.ID, ModelKey: m.Key, ModelName: m.Name, Vote: models.VotePending,
			})
		}
		out = append(out, bw)
	}
	return out, nil
}

// MissingVoteModels lists catalog models that have no successful verdict yet
// for the given bill.
func (s *Store) MissingVoteModels(ctx context.Context, billID string) ([]models.Model, error) {
	byBill, err := s.VotesByBill(ctx, billID)
	if err != nil {
		return nil, err
	}
	var out []models.Model
	for _, m := range models.Catalog {
		if v, ok := byBill[billID][m.Key]; ok && v.Decided() {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

// BillIDsMissingVotes lists bills that still need at least one model verdict,
// newest first.
func (s *Store) BillIDsMissingVotes(ctx context.Context) ([]string, error) {
	bills, err := s.Bills(ctx)
	if err != nil {
		return nil, err
	}
	byBill, err := s.VotesByBill(ctx)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, b := range bills {
		decided := 0
		for _, v := range byBill[b.ID] {
			if v.Decided() {
				decided++
			}
		}
		if decided < len(models.Catalog) {
			out = append(out, b.ID)
		}
	}
	return out, nil
}

// BillsNeedingIdeology lists bills whose ideology score has not been set.
func (s *Store) BillsNeedingIdeology(ctx context.Context) ([]models.Bill, error) {
	bills, err := s.Bills(ctx)
	if err != nil {
		return nil, err
	}
	var out []models.Bill
	for _, b := range bills {
		if b.IdeologyScore == 0 {
			out = append(out, b)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().Unix()
}

func fromUnix(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(v, 0).UTC()
}
