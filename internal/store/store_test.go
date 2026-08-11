package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func statuteBill(id string) models.Bill {
	return models.Bill{
		ID:         id,
		Number:     "S. 1264",
		Title:      "Border Security and Asylum Reform Act of 2025",
		Chamber:    models.ChamberSenate,
		Summary:    "A CRS summary.",
		FullText:   `<section><enum>1.</enum><header>Short title</header></section>`,
		TextSource: models.TextSourceCongressXML,
	}
}

func TestPurgeMalformedVotesReopensThemForCollection(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if _, err := st.UpsertBill(ctx, models.Bill{ID: "s-1264", Number: "S. 1264", Title: "Test", Chamber: models.ChamberSenate}); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}

	votes := []models.Vote{
		{BillID: "s-1264", ModelKey: "opus", Vote: models.VoteYes, Reason: "Strengthens border security while preserving due process."},
		// The shape that leaked onto the page for Gemini.
		{BillID: "s-1264", ModelKey: "gemini", Vote: models.VoteYes, Reason: `{"vote": "Yes", "reason": "This.`},
		{BillID: "s-1264", ModelKey: "grok", Vote: models.VoteNo, Reason: "```json"},
	}
	for _, v := range votes {
		if err := st.SaveVote(ctx, v); err != nil {
			t.Fatalf("SaveVote: %v", err)
		}
	}

	purged, err := st.PurgeMalformedVotes(ctx)
	if err != nil {
		t.Fatalf("PurgeMalformedVotes: %v", err)
	}
	if purged != 2 {
		t.Errorf("purged %d verdicts, want 2", purged)
	}

	stored, err := st.VotesByBill(ctx, "s-1264")
	if err != nil {
		t.Fatalf("VotesByBill: %v", err)
	}
	if _, ok := stored["s-1264"]["opus"]; !ok {
		t.Error("the clean verdict should have survived")
	}
	if _, ok := stored["s-1264"]["gemini"]; ok {
		t.Error("the malformed verdict should have been removed")
	}

	missing, err := st.MissingVoteModels(ctx, "s-1264")
	if err != nil {
		t.Fatalf("MissingVoteModels: %v", err)
	}
	for _, want := range []string{"gemini", "grok"} {
		found := false
		for _, m := range missing {
			if m.Key == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s should be queued for re-collection", want)
		}
	}
}

func TestPurgeLeavesCleanVerdictsAlone(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if _, err := st.UpsertBill(ctx, models.Bill{ID: "hr-1", Number: "H.R. 1", Chamber: models.ChamberHouse}); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	// A rationale may legitimately mention braces or the word reason.
	if err := st.SaveVote(ctx, models.Vote{
		BillID: "hr-1", ModelKey: "opus", Vote: models.VoteNo,
		Reason: "The main reason is cost, which sec. 4 {as drafted} leaves unbounded.",
	}); err != nil {
		t.Fatalf("SaveVote: %v", err)
	}

	purged, err := st.PurgeMalformedVotes(ctx)
	if err != nil {
		t.Fatalf("PurgeMalformedVotes: %v", err)
	}
	if purged != 0 {
		t.Errorf("purged %d verdicts, want 0", purged)
	}
}

func TestDeliberationRoundTrip(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	bill := statuteBill("s-1264")
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}

	want := models.Deliberation{
		BillID:   bill.ID,
		ModelKey: "opus",
		Pros:     []string{"Sec. 3 funds 3,000 additional agents.", "Sec. 7 preserves access to counsel."},
		Cons:     []string{"Sec. 5 raises the credible fear standard sharply."},
		Digest:   "1. Sec. 1. Short title\nNames the Act.",
		Sections: 4,
		Overflow: true,
		TextHash: bill.TextFingerprint(),
	}
	if err := st.SaveDeliberation(ctx, want); err != nil {
		t.Fatalf("SaveDeliberation: %v", err)
	}

	got, err := st.Deliberation(ctx, bill.ID, "opus")
	if err != nil {
		t.Fatalf("Deliberation: %v", err)
	}
	if len(got.Pros) != 2 || got.Pros[0] != want.Pros[0] {
		t.Errorf("pros = %v, want %v", got.Pros, want.Pros)
	}
	if len(got.Cons) != 1 || got.Cons[0] != want.Cons[0] {
		t.Errorf("cons = %v, want %v", got.Cons, want.Cons)
	}
	if got.Digest != want.Digest || got.Sections != 4 || !got.Overflow {
		t.Errorf("section notes did not survive: %+v", got)
	}
	if got.TextHash != want.TextHash {
		t.Errorf("text hash = %q, want %q", got.TextHash, want.TextHash)
	}
	if got.ModelName != "Opus" {
		t.Errorf("model name = %q, want it resolved from the catalog", got.ModelName)
	}
	if got.CreatedAt.IsZero() {
		t.Error("created_at was not stamped")
	}

	if _, err := st.Deliberation(ctx, bill.ID, "grok"); err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound for a model that has not deliberated", err)
	}
}

// The pros and cons a model wrote have to reach the UI alongside its verdict.
func TestWithVotesCarriesTheModelsOwnProsAndCons(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	bill := statuteBill("s-1264")
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	if err := st.SaveVote(ctx, models.Vote{BillID: bill.ID, ModelKey: "opus", Vote: models.VoteYes, Reason: "Worth it."}); err != nil {
		t.Fatalf("SaveVote: %v", err)
	}
	if err := st.SaveDeliberation(ctx, models.Deliberation{
		BillID: bill.ID, ModelKey: "opus", Pros: []string{"Funds it."}, Cons: []string{"Vague."},
	}); err != nil {
		t.Fatalf("SaveDeliberation: %v", err)
	}

	got, err := st.WithVotes(ctx, []models.Bill{bill})
	if err != nil {
		t.Fatalf("WithVotes: %v", err)
	}
	for _, v := range got[0].Votes {
		switch v.ModelKey {
		case "opus":
			if len(v.Pros) != 1 || len(v.Cons) != 1 {
				t.Errorf("opus verdict lost its memo: pros=%v cons=%v", v.Pros, v.Cons)
			}
		default:
			if len(v.Pros) != 0 || len(v.Cons) != 0 {
				t.Errorf("%s has no memo but carries pros=%v cons=%v", v.ModelKey, v.Pros, v.Cons)
			}
		}
	}
}

// Newly published statute text invalidates the reading of the old text.
func TestUpsertBillReportsChangedStatuteText(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	bill := statuteBill("s-1264")

	if changed, err := st.UpsertBill(ctx, bill); err != nil || changed {
		t.Fatalf("first insert: changed=%v err=%v, want changed=false", changed, err)
	}
	if changed, err := st.UpsertBill(ctx, bill); err != nil || changed {
		t.Fatalf("re-upsert of identical text: changed=%v err=%v", changed, err)
	}

	bill.Status = "Passed Senate"
	if changed, err := st.UpsertBill(ctx, bill); err != nil || changed {
		t.Fatalf("a new latest action is not a text change: changed=%v err=%v", changed, err)
	}

	engrossed := bill
	engrossed.FullText += `<section><enum>2.</enum><header>Appropriations</header></section>`
	changed, err := st.UpsertBill(ctx, engrossed)
	if err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	if !changed {
		t.Error("a newly published text version must be reported as changed")
	}

	stored, err := st.Bill(ctx, bill.ID)
	if err != nil {
		t.Fatalf("Bill: %v", err)
	}
	if stored.FullText != engrossed.FullText || !stored.HasStatuteText() {
		t.Errorf("stored text = %q", stored.FullText)
	}
}

func TestBillTextProvenanceIsPersisted(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	bill := statuteBill("hr-3076")
	bill.TextFormat = "Formatted XML"
	bill.TextVersion = "Reported in House"
	bill.TextURL = "https://www.congress.gov/117/bills/hr3076/BILLS-117hr3076rh.xml"
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}

	got, err := st.Bill(ctx, bill.ID)
	if err != nil {
		t.Fatalf("Bill: %v", err)
	}
	if got.TextSource != models.TextSourceCongressXML || got.TextFormat != "Formatted XML" ||
		got.TextVersion != "Reported in House" || got.TextURL != bill.TextURL {
		t.Errorf("provenance lost: %+v", got)
	}
}

// A database written by the summary-era build has to be cleared once, so the
// bills are re-ingested as statute text and re-voted.
func TestMigrationDiscardsSummaryEraCorpus(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")

	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	_, err = legacy.Exec(`
CREATE TABLE bills (
    id TEXT PRIMARY KEY, number TEXT NOT NULL, title TEXT NOT NULL, chamber TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT '', status_category TEXT NOT NULL DEFAULT '',
    policy_area TEXT NOT NULL DEFAULT '', sponsor TEXT NOT NULL DEFAULT '',
    sponsor_party TEXT NOT NULL DEFAULT '', summary TEXT NOT NULL DEFAULT '',
    full_text TEXT NOT NULL DEFAULT '', ideology_score REAL NOT NULL DEFAULT 0,
    source TEXT NOT NULL DEFAULT 'seed', source_url TEXT NOT NULL DEFAULT '',
    introduced_date INTEGER NOT NULL DEFAULT 0, updated_at INTEGER NOT NULL DEFAULT 0);
CREATE TABLE votes (
    bill_id TEXT NOT NULL, model_key TEXT NOT NULL, vote TEXT NOT NULL,
    reason TEXT NOT NULL DEFAULT '', error TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT 0, PRIMARY KEY (bill_id, model_key));
INSERT INTO bills (id, number, title, chamber, summary, full_text)
VALUES ('s-1264', 'S. 1264', 'Old', 'Senate', 'A CRS summary.', 'A CRS summary.');
INSERT INTO votes (bill_id, model_key, vote, reason) VALUES ('s-1264', 'opus', 'Yes', 'Read the summary.');
INSERT INTO votes (bill_id, model_key, vote, reason) VALUES ('s-1264', 'grok', 'No', 'Read the summary.');`)
	if err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	legacy.Close()

	st, discarded, err := OpenWithMigration(path)
	if err != nil {
		t.Fatalf("OpenWithMigration: %v", err)
	}
	defer st.Close()
	if discarded != 2 {
		t.Errorf("discarded %d summary-era verdict(s), want 2", discarded)
	}
	if n, err := st.CountBills(ctx); err != nil || n != 0 {
		t.Errorf("bills = %d (err %v), want the summary-era corpus cleared for re-ingest", n, err)
	}

	// The new columns exist and the version is recorded, so reopening is a
	// no-op rather than a second wipe.
	if _, err := st.UpsertBill(ctx, statuteBill("s-1264")); err != nil {
		t.Fatalf("UpsertBill after migration: %v", err)
	}
	if err := st.SaveVote(ctx, models.Vote{BillID: "s-1264", ModelKey: "opus", Vote: models.VoteYes, Reason: "Read the law."}); err != nil {
		t.Fatalf("SaveVote: %v", err)
	}
	st.Close()

	again, discarded, err := OpenWithMigration(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer again.Close()
	if discarded != 0 {
		t.Errorf("reopening discarded %d verdict(s), want 0", discarded)
	}
	if n, err := again.CountBills(ctx); err != nil || n != 1 {
		t.Errorf("bills = %d (err %v), want the statute-era corpus kept", n, err)
	}
}

func memberVote(billID, bioguide, position string, day int) models.MemberVote {
	return models.MemberVote{
		BillID:   billID,
		Bioguide: bioguide,
		Chamber:  models.ChamberHouse,
		Position: position,
		RollCall: "house-2026-283",
		Question: "On Passage",
		VotedAt:  time.Date(2026, 7, day, 0, 0, 0, 0, time.UTC),
	}
}

// A member can be recorded on the same bill twice — passage, and then
// concurring in the other chamber's text — and the later vote is the position
// that stands.
func TestMemberVotesKeepTheLatestPosition(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if _, err := st.UpsertBill(ctx, statuteBill("hr-8884-119")); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	for _, mv := range []models.MemberVote{
		memberVote("hr-8884-119", "O000172", models.PositionYea, 23),
		memberVote("hr-8884-119", "O000172", models.PositionNay, 30),
		memberVote("hr-8884-119", "O000172", models.PositionPresent, 1),
	} {
		if err := st.SaveMemberVote(ctx, mv); err != nil {
			t.Fatalf("SaveMemberVote: %v", err)
		}
	}

	stored, err := st.MemberVotes(ctx)
	if err != nil {
		t.Fatalf("MemberVotes: %v", err)
	}
	if len(stored) != 1 {
		t.Fatalf("got %d rows, want one per member and bill", len(stored))
	}
	if stored[0].Position != models.PositionNay {
		t.Errorf("position = %q, want the July 30 vote to stand over the earlier one", stored[0].Position)
	}
	if !stored[0].Decided() {
		t.Error("a Nay is a side")
	}
}

// A window that moves on leaves positions behind for bills that are no longer
// tracked. They are dropped, and so is the record of having read the roll call
// they came from — otherwise a bill that comes back into the window is skipped
// as already seen while carrying no positions at all.
func TestFloorVotesForDroppedBillsArePurged(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if _, err := st.UpsertBill(ctx, statuteBill("hr-8884-119")); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	for _, mv := range []models.MemberVote{
		memberVote("hr-8884-119", "O000172", models.PositionYea, 23),
		memberVote("hr-1-118", "M001184", models.PositionNay, 23),
	} {
		if err := st.SaveMemberVote(ctx, mv); err != nil {
			t.Fatalf("SaveMemberVote: %v", err)
		}
	}
	for _, rc := range []RollCall{
		{ID: "house-2026-283", BillID: "hr-8884-119"},
		{ID: "house-2024-100", BillID: "hr-1-118"},
	} {
		if err := st.SaveRollCall(ctx, rc); err != nil {
			t.Fatalf("SaveRollCall: %v", err)
		}
	}

	dropped, err := st.PurgeOrphanFloorVotes(ctx)
	if err != nil {
		t.Fatalf("PurgeOrphanFloorVotes: %v", err)
	}
	if dropped != 1 {
		t.Errorf("dropped %d, want the one orphan", dropped)
	}
	stored, err := st.MemberVotes(ctx)
	if err != nil || len(stored) != 1 || stored[0].BillID != "hr-8884-119" {
		t.Errorf("remaining = %+v (%v)", stored, err)
	}
	known, err := st.KnownRollCalls(ctx)
	if err != nil || known["house-2024-100"] || !known["house-2026-283"] {
		t.Errorf("known roll calls = %v (%v)", known, err)
	}
}

func TestRollCallsAreOnlyReadOnce(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	known, err := st.KnownRollCalls(ctx)
	if err != nil || len(known) != 0 {
		t.Fatalf("a fresh database knows no roll calls: %v %v", known, err)
	}
	rc := RollCall{ID: "house-2026-283", Chamber: models.ChamberHouse, BillID: "hr-8884-119",
		Question: "On Passage", VotedAt: time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC), Members: 3}
	if err := st.SaveRollCall(ctx, rc); err != nil {
		t.Fatalf("SaveRollCall: %v", err)
	}
	if err := st.SaveRollCall(ctx, rc); err != nil {
		t.Fatalf("re-recording a roll call should be harmless: %v", err)
	}
	known, err = st.KnownRollCalls(ctx)
	if err != nil || !known["house-2026-283"] || len(known) != 1 {
		t.Errorf("known = %v (%v)", known, err)
	}
}

// The agreement table is served from the cache only while the verdicts and
// floor votes behind it are the ones it was computed from.
func TestContenderCacheIsKeyedToItsInputs(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if _, err := st.CachedContenders(ctx, "sig-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an empty cache should miss, got %v", err)
	}
	if err := st.SaveContenders(ctx, "sig-1", []byte(`{"models":[]}`)); err != nil {
		t.Fatalf("SaveContenders: %v", err)
	}
	payload, err := st.CachedContenders(ctx, "sig-1")
	if err != nil || string(payload) != `{"models":[]}` {
		t.Errorf("cached payload = %q (%v)", payload, err)
	}

	// New inputs mean a new signature, and the entry they replace is not worth
	// keeping: nothing will ever ask for it again.
	if err := st.SaveContenders(ctx, "sig-2", []byte(`{"models":[1]}`)); err != nil {
		t.Fatalf("SaveContenders: %v", err)
	}
	if _, err := st.CachedContenders(ctx, "sig-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("the superseded entry should be gone, got %v", err)
	}
}

func TestContenderInputsSummariseTheData(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if _, err := st.UpsertBill(ctx, statuteBill("hr-8884-119")); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	if err := st.SaveVote(ctx, models.Vote{BillID: "hr-8884-119", ModelKey: "opus", Vote: models.VoteYes,
		CreatedAt: time.Unix(1700, 0)}); err != nil {
		t.Fatalf("SaveVote: %v", err)
	}
	if err := st.SaveMemberVote(ctx, memberVote("hr-8884-119", "O000172", models.PositionYea, 23)); err != nil {
		t.Fatalf("SaveMemberVote: %v", err)
	}

	in, err := st.ContenderInputs(ctx)
	if err != nil {
		t.Fatalf("ContenderInputs: %v", err)
	}
	if in.ModelVotes != 1 || in.ModelVotesAt != 1700 {
		t.Errorf("model verdicts = %d at %d", in.ModelVotes, in.ModelVotesAt)
	}
	if in.MemberVotes != 1 || in.MemberVotesAt == 0 {
		t.Errorf("floor votes = %d at %d", in.MemberVotes, in.MemberVotesAt)
	}
}
