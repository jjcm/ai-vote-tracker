// Package store persists bills and model votes in SQLite so that OpenRouter is
// only billed once per (bill, model) pair.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
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

// schemaVersion is bumped when stored model output stops being comparable with
// what the current pipeline produces. Version 2 is the move from CRS summaries
// to statute text: verdicts written against a summary are not verdicts on the
// law, so the corpus and every verdict on it are discarded once.
const schemaVersion = 2

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
    text_source     TEXT NOT NULL DEFAULT '',
    text_format     TEXT NOT NULL DEFAULT '',
    text_version    TEXT NOT NULL DEFAULT '',
    text_url        TEXT NOT NULL DEFAULT '',
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

-- One model's private working papers on one bill: the pros and cons memo it
-- wrote before voting, and the section notes it took if the statute text was
-- too large to read in a single pass.
CREATE TABLE IF NOT EXISTS deliberations (
    bill_id    TEXT NOT NULL,
    model_key  TEXT NOT NULL,
    pros       TEXT NOT NULL DEFAULT '[]',
    cons       TEXT NOT NULL DEFAULT '[]',
    digest     TEXT NOT NULL DEFAULT '',
    sections   INTEGER NOT NULL DEFAULT 0,
    overflow   INTEGER NOT NULL DEFAULT 0,
    text_hash  TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (bill_id, model_key),
    FOREIGN KEY (bill_id) REFERENCES bills(id) ON DELETE CASCADE
);

-- How one member of Congress voted on the floor on one bill. Only members on
-- the 2028 watch list are stored: this table exists to compare models against
-- them, not to mirror the Clerk. A member can be recorded on the same bill
-- twice — passage, then concurring in the other chamber's text — and the later
-- vote is the one kept, so the row is keyed by bill and member.
CREATE TABLE IF NOT EXISTS member_votes (
    bill_id   TEXT NOT NULL,
    bioguide  TEXT NOT NULL,
    chamber   TEXT NOT NULL DEFAULT '',
    position  TEXT NOT NULL,
    roll_call TEXT NOT NULL DEFAULT '',
    question  TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    voted_at  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (bill_id, bioguide)
);

-- Roll calls whose member positions have already been read, so a backfill that
-- runs again does not download them a second time.
CREATE TABLE IF NOT EXISTS roll_calls (
    id         TEXT PRIMARY KEY,
    chamber    TEXT NOT NULL DEFAULT '',
    bill_id    TEXT NOT NULL DEFAULT '',
    question   TEXT NOT NULL DEFAULT '',
    voted_at   INTEGER NOT NULL DEFAULT 0,
    members    INTEGER NOT NULL DEFAULT 0,
    fetched_at INTEGER NOT NULL DEFAULT 0
);

-- The computed model-to-member agreement table, cached against a signature of
-- the model verdicts and member votes it was derived from. A change to either
-- changes the signature, which is what makes the entry stale.
CREATE TABLE IF NOT EXISTS contender_matches (
    signature  TEXT PRIMARY KEY,
    payload    TEXT NOT NULL,
    created_at INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_bills_updated ON bills(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_votes_model ON votes(model_key);
CREATE INDEX IF NOT EXISTS idx_member_votes_member ON member_votes(bioguide);
`

// Open creates (or opens) the database at path, applies the schema, and
// migrates it. Migrated reports how many stale verdicts the migration dropped.
func Open(path string) (*Store, error) {
	st, _, err := OpenWithMigration(path)
	return st, err
}

// OpenWithMigration is Open, reporting what the schema migration discarded so
// the caller can log it.
func OpenWithMigration(path string) (*Store, int, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, 0, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, 0, fmt.Errorf("open database: %w", err)
	}
	// modernc's driver serialises fine, but a single writer avoids SQLITE_BUSY
	// churn when the vote workers all finish at once.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, 0, fmt.Errorf("apply schema: %w", err)
	}
	discarded, err := migrate(db)
	if err != nil {
		db.Close()
		return nil, 0, err
	}
	return &Store{db: db}, discarded, nil
}

// migrate brings an older database up to the current schema: it adds columns
// that predate it, and on a version bump clears the corpus so that bills
// ingested as CRS summaries are re-fetched as statute text and re-voted.
func migrate(db *sql.DB) (discarded int, err error) {
	added := map[string]string{
		"text_source":  `TEXT NOT NULL DEFAULT ''`,
		"text_format":  `TEXT NOT NULL DEFAULT ''`,
		"text_version": `TEXT NOT NULL DEFAULT ''`,
		"text_url":     `TEXT NOT NULL DEFAULT ''`,
	}
	present, err := columns(db, "bills")
	if err != nil {
		return 0, err
	}
	for name, decl := range added {
		if present[name] {
			continue
		}
		if _, err := db.Exec(`ALTER TABLE bills ADD COLUMN ` + name + ` ` + decl); err != nil {
			return 0, fmt.Errorf("add bills.%s: %w", name, err)
		}
	}

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	if version >= schemaVersion {
		return 0, nil
	}

	if err := db.QueryRow(`SELECT COUNT(*) FROM votes`).Scan(&discarded); err != nil {
		return 0, err
	}
	// Deleting the bills cascades to their verdicts and deliberations.
	for _, stmt := range []string{`DELETE FROM deliberations`, `DELETE FROM votes`, `DELETE FROM bills`} {
		if _, err := db.Exec(stmt); err != nil {
			return 0, fmt.Errorf("%s: %w", stmt, err)
		}
	}
	// PRAGMA does not accept a bound parameter.
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion)); err != nil {
		return 0, fmt.Errorf("set schema version: %w", err)
	}
	return discarded, nil
}

func columns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, fmt.Errorf("read %s columns: %w", table, err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
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
// ideology score when the incoming bill does not carry one. It reports whether
// the statute text changed, which invalidates every model's reading of it.
func (s *Store) UpsertBill(ctx context.Context, b models.Bill) (textChanged bool, err error) {
	var storedText, storedSource string
	err = s.db.QueryRowContext(ctx, `SELECT full_text, text_source FROM bills WHERE id = ?`, b.ID).
		Scan(&storedText, &storedSource)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		textChanged = false
	case err != nil:
		return false, err
	default:
		textChanged = storedText != b.FullText || storedSource != b.TextSource
	}

	_, err = s.db.ExecContext(ctx, `
INSERT INTO bills (id, number, title, chamber, status, status_category, policy_area, sponsor,
                   sponsor_party, summary, full_text, text_source, text_format, text_version,
                   text_url, ideology_score, source, source_url, introduced_date, updated_at)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
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
    text_source=excluded.text_source,
    text_format=excluded.text_format,
    text_version=excluded.text_version,
    text_url=excluded.text_url,
    ideology_score=CASE WHEN excluded.ideology_score = 0 THEN bills.ideology_score ELSE excluded.ideology_score END,
    source=excluded.source,
    source_url=excluded.source_url,
    introduced_date=excluded.introduced_date,
    updated_at=excluded.updated_at`,
		b.ID, b.Number, b.Title, b.Chamber, b.Status, b.StatusCategory, b.PolicyArea, b.Sponsor,
		b.SponsorParty, b.Summary, b.FullText, b.TextSource, b.TextFormat, b.TextVersion,
		b.TextURL, b.IdeologyScore, b.Source, b.SourceURL,
		unix(b.IntroducedDate), unix(b.UpdatedAt))
	return textChanged, err
}

// SetIdeologyScore stores a computed ideology score for a bill.
func (s *Store) SetIdeologyScore(ctx context.Context, billID string, score float64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE bills SET ideology_score = ? WHERE id = ?`, score, billID)
	return err
}

const billColumns = `id, number, title, chamber, status, status_category, policy_area, sponsor,
	sponsor_party, summary, full_text, text_source, text_format, text_version, text_url,
	ideology_score, source, source_url, introduced_date, updated_at`

func scanBill(rows interface{ Scan(...any) error }) (models.Bill, error) {
	var b models.Bill
	var introduced, updated int64
	err := rows.Scan(&b.ID, &b.Number, &b.Title, &b.Chamber, &b.Status, &b.StatusCategory,
		&b.PolicyArea, &b.Sponsor, &b.SponsorParty, &b.Summary, &b.FullText, &b.TextSource,
		&b.TextFormat, &b.TextVersion, &b.TextURL, &b.IdeologyScore,
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

// PurgeMalformedVotes drops verdicts whose rationale still carries the response
// envelope, which an earlier parsing bug could store. Removing them puts the
// (bill, model) pair back in the missing set so the next round re-collects it.
func (s *Store) PurgeMalformedVotes(ctx context.Context) (int, error) {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM votes
WHERE reason LIKE ?
   OR reason LIKE ?
   OR reason LIKE ?
   OR reason LIKE ?`,
		"{%", `%"vote"%`, `%"reason"%`, "%```%")
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// DeleteVotes clears all recorded verdicts for a bill.
func (s *Store) DeleteVotes(ctx context.Context, billID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM votes WHERE bill_id = ?`, billID)
	return err
}

// SaveDeliberation records one model's pros and cons memo, and the section
// notes behind it, so a re-vote reads the same memo instead of paying for a
// fresh one.
func (s *Store) SaveDeliberation(ctx context.Context, d models.Deliberation) error {
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	pros, err := json.Marshal(nonNil(d.Pros))
	if err != nil {
		return err
	}
	cons, err := json.Marshal(nonNil(d.Cons))
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO deliberations (bill_id, model_key, pros, cons, digest, sections, overflow, text_hash, created_at)
VALUES (?,?,?,?,?,?,?,?,?)
ON CONFLICT(bill_id, model_key) DO UPDATE SET
    pros=excluded.pros, cons=excluded.cons, digest=excluded.digest, sections=excluded.sections,
    overflow=excluded.overflow, text_hash=excluded.text_hash, created_at=excluded.created_at`,
		d.BillID, d.ModelKey, string(pros), string(cons), d.Digest, d.Sections, d.Overflow,
		d.TextHash, unix(d.CreatedAt))
	return err
}

// Deliberation loads one model's memo for one bill.
func (s *Store) Deliberation(ctx context.Context, billID, modelKey string) (models.Deliberation, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT bill_id, model_key, pros, cons, digest, sections, overflow, text_hash, created_at
FROM deliberations WHERE bill_id = ? AND model_key = ?`, billID, modelKey)
	d, err := scanDeliberation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Deliberation{}, ErrNotFound
	}
	return d, err
}

// DeliberationsByBill returns memos keyed by bill id then model key.
func (s *Store) DeliberationsByBill(ctx context.Context, billIDs ...string) (map[string]map[string]models.Deliberation, error) {
	query := `SELECT bill_id, model_key, pros, cons, digest, sections, overflow, text_hash, created_at FROM deliberations`
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

	out := map[string]map[string]models.Deliberation{}
	for rows.Next() {
		d, err := scanDeliberation(rows)
		if err != nil {
			return nil, err
		}
		if out[d.BillID] == nil {
			out[d.BillID] = map[string]models.Deliberation{}
		}
		out[d.BillID][d.ModelKey] = d
	}
	return out, rows.Err()
}

// DeleteDeliberations clears every model's memo for a bill, which is what
// newly published statute text calls for.
func (s *Store) DeleteDeliberations(ctx context.Context, billID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM deliberations WHERE bill_id = ?`, billID)
	return err
}

func scanDeliberation(row interface{ Scan(...any) error }) (models.Deliberation, error) {
	var d models.Deliberation
	var pros, cons string
	var created int64
	if err := row.Scan(&d.BillID, &d.ModelKey, &pros, &cons, &d.Digest, &d.Sections,
		&d.Overflow, &d.TextHash, &created); err != nil {
		return models.Deliberation{}, err
	}
	// A memo whose lists cannot be decoded is treated as absent rather than
	// fatal: the model simply writes it again.
	_ = json.Unmarshal([]byte(pros), &d.Pros)
	_ = json.Unmarshal([]byte(cons), &d.Cons)
	d.CreatedAt = fromUnix(created)
	if m, ok := models.ModelByKey(d.ModelKey); ok {
		d.ModelName = m.Name
	}
	return d, nil
}

func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
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
// Each verdict carries the pros and cons the same model wrote before casting it.
func (s *Store) WithVotes(ctx context.Context, bills []models.Bill) ([]models.BillWithVotes, error) {
	ids := make([]string, 0, len(bills))
	for _, b := range bills {
		ids = append(ids, b.ID)
	}
	byBill := map[string]map[string]models.Vote{}
	memos := map[string]map[string]models.Deliberation{}
	if len(ids) > 0 {
		var err error
		byBill, err = s.VotesByBill(ctx, ids...)
		if err != nil {
			return nil, err
		}
		memos, err = s.DeliberationsByBill(ctx, ids...)
		if err != nil {
			return nil, err
		}
	}

	out := make([]models.BillWithVotes, 0, len(bills))
	for _, b := range bills {
		bw := models.BillWithVotes{Bill: b, Votes: make([]models.Vote, 0, len(models.Catalog))}
		for _, m := range models.Catalog {
			if v, ok := byBill[b.ID][m.Key]; ok {
				if memo, ok := memos[b.ID][m.Key]; ok {
					v.Pros, v.Cons = memo.Pros, memo.Cons
				}
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

// SaveMemberVote records one member's floor position on one bill. A member can
// be recorded on the same bill more than once — passage and then concurring in
// the other chamber's amendment — and the later roll call is the one that
// stands, so an older one never overwrites a newer one.
func (s *Store) SaveMemberVote(ctx context.Context, v models.MemberVote) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO member_votes (bill_id, bioguide, chamber, position, roll_call, question, source_url, voted_at)
VALUES (?,?,?,?,?,?,?,?)
ON CONFLICT(bill_id, bioguide) DO UPDATE SET
    chamber=excluded.chamber,
    position=excluded.position,
    roll_call=excluded.roll_call,
    question=excluded.question,
    source_url=excluded.source_url,
    voted_at=excluded.voted_at
WHERE excluded.voted_at >= member_votes.voted_at`,
		v.BillID, v.Bioguide, v.Chamber, v.Position, v.RollCall, v.Question, v.SourceURL, unix(v.VotedAt))
	return err
}

// MemberVotes returns every stored floor position.
func (s *Store) MemberVotes(ctx context.Context) ([]models.MemberVote, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT bill_id, bioguide, chamber, position, roll_call, question, source_url, voted_at
FROM member_votes ORDER BY voted_at DESC, bill_id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.MemberVote
	for rows.Next() {
		var v models.MemberVote
		var votedAt int64
		if err := rows.Scan(&v.BillID, &v.Bioguide, &v.Chamber, &v.Position, &v.RollCall,
			&v.Question, &v.SourceURL, &votedAt); err != nil {
			return nil, err
		}
		v.VotedAt = fromUnix(votedAt)
		out = append(out, v)
	}
	return out, rows.Err()
}

// PurgeOrphanFloorVotes drops the stored positions for bills that are no
// longer in the corpus, so a moving window does not leave orphans behind. The
// roll calls they came from are forgotten with them: a bill that comes back
// into the window has to be read again, and would otherwise be skipped as
// already seen while carrying no positions at all.
func (s *Store) PurgeOrphanFloorVotes(ctx context.Context) (int, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM member_votes WHERE bill_id NOT IN (SELECT id FROM bills)`)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM roll_calls WHERE bill_id NOT IN (SELECT id FROM bills)`); err != nil {
		return 0, err
	}
	return int(n), nil
}

// RollCall is the record that one roll call's member positions have been read.
type RollCall struct {
	ID       string
	Chamber  string
	BillID   string
	Question string
	VotedAt  time.Time
	Members  int
}

// SaveRollCall marks a roll call as ingested.
func (s *Store) SaveRollCall(ctx context.Context, rc RollCall) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO roll_calls (id, chamber, bill_id, question, voted_at, members, fetched_at)
VALUES (?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
    chamber=excluded.chamber, bill_id=excluded.bill_id, question=excluded.question,
    voted_at=excluded.voted_at, members=excluded.members, fetched_at=excluded.fetched_at`,
		rc.ID, rc.Chamber, rc.BillID, rc.Question, unix(rc.VotedAt), rc.Members,
		time.Now().UTC().Unix())
	return err
}

// KnownRollCalls returns the ids of roll calls already read.
func (s *Store) KnownRollCalls(ctx context.Context) (map[string]bool, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM roll_calls`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// ContenderInputs is the shape of the data an agreement table was computed
// from: enough to tell a cached table from a stale one without recomputing it.
type ContenderInputs struct {
	ModelVotes    int
	ModelVotesAt  int64
	MemberVotes   int
	MemberVotesAt int64
}

// ContenderInputs summarises the model verdicts and member positions on hand.
func (s *Store) ContenderInputs(ctx context.Context) (ContenderInputs, error) {
	var in ContenderInputs
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(created_at), 0) FROM votes`).Scan(&in.ModelVotes, &in.ModelVotesAt)
	if err != nil {
		return in, err
	}
	err = s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(MAX(voted_at), 0) FROM member_votes`).Scan(&in.MemberVotes, &in.MemberVotesAt)
	return in, err
}

// CachedContenders returns the agreement table stored under a signature.
func (s *Store) CachedContenders(ctx context.Context, signature string) ([]byte, error) {
	var payload string
	err := s.db.QueryRowContext(ctx,
		`SELECT payload FROM contender_matches WHERE signature = ?`, signature).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return []byte(payload), err
}

// SaveContenders caches an agreement table, dropping the entries computed from
// data that has since moved on. Only the current signature is worth keeping:
// nothing ever asks for an older one.
func (s *Store) SaveContenders(ctx context.Context, signature string, payload []byte) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM contender_matches WHERE signature <> ?`, signature); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO contender_matches (signature, payload, created_at) VALUES (?,?,?)
ON CONFLICT(signature) DO UPDATE SET payload=excluded.payload, created_at=excluded.created_at`,
		signature, string(payload), time.Now().UTC().Unix())
	return err
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
