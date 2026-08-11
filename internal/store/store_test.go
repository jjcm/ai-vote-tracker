package store

import (
	"context"
	"path/filepath"
	"testing"

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

func TestPurgeMalformedVotesReopensThemForCollection(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if err := st.UpsertBill(ctx, models.Bill{ID: "s-1264", Number: "S. 1264", Title: "Test", Chamber: models.ChamberSenate}); err != nil {
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

	if err := st.UpsertBill(ctx, models.Bill{ID: "hr-1", Number: "H.R. 1", Chamber: models.ChamberHouse}); err != nil {
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
