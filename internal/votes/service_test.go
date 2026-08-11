package votes

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/congress"
	"github.com/pwnies/ai-vote-tracker/internal/models"
	"github.com/pwnies/ai-vote-tracker/internal/openrouter"
	"github.com/pwnies/ai-vote-tracker/internal/store"
)

// router counts the calls the service makes at each stage of the pipeline, so
// the tests can tell a memo that was written from one that was reused.
type router struct {
	mu     sync.Mutex
	stages map[string]int
}

func (r *router) count(stage string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stages[stage]
}

func newService(t *testing.T) (*Service, *store.Store, *router) {
	t.Helper()
	r := &router{stages: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(req.Body).Decode(&body)
		var system string
		for _, m := range body.Messages {
			if m.Role == "system" {
				system = m.Content
			}
		}

		// The floor vote is matched first: its prompt also mentions the memo.
		stage, content := "unknown", `{}`
		switch {
		case strings.Contains(system, "SECTION NOTE"):
			stage, content = "section", `{"note": "Authorizes appropriations and sets a deadline."}`
		case strings.Contains(system, "recorded floor vote"):
			stage, content = "vote", `{"vote": "Yes", "reason": "Funds the program with workable deadlines."}`
		case strings.Contains(system, "pros and cons memo"):
			stage, content = "memo", `{"pros": ["Sec. 2 funds it."], "cons": ["Sec. 5 is vague."]}`
		case strings.Contains(system, "ideological direction"):
			stage, content = "ideology", `{"score": 0.3}`
		}
		r.mu.Lock()
		r.stages[stage]++
		r.mu.Unlock()

		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
		})
	}))
	t.Cleanup(srv.Close)

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	client := openrouter.New(srv.URL, "test-key", "", 5*time.Second)
	svc := New(st, client, congress.New("", ""), 4, log.New(io.Discard, "", 0))
	return svc, st, r
}

func testBill() models.Bill {
	return models.Bill{
		ID:         "s-1264",
		Number:     "S. 1264",
		Title:      "Border Security and Asylum Reform Act of 2025",
		Chamber:    models.ChamberSenate,
		Summary:    "A CRS summary, for the bill card only.",
		FullText:   `<section><enum>1.</enum><header>Short title</header><text>This Act may be cited as the Testing Act.</text></section>`,
		TextSource: models.TextSourceCongressXML,
	}
}

// Every model writes its own memo and then votes on it, and both are stored.
func TestVoteBillRecordsAMemoAndAVoteForEveryModel(t *testing.T) {
	ctx := context.Background()
	svc, st, r := newService(t)
	bill := testBill()
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}

	if err := svc.VoteBill(ctx, bill.ID, false); err != nil {
		t.Fatalf("VoteBill: %v", err)
	}

	if got, want := r.count("memo"), len(models.Catalog); got != want {
		t.Errorf("%d memo call(s), want one per model (%d)", got, want)
	}
	if got, want := r.count("vote"), len(models.Catalog); got != want {
		t.Errorf("%d vote call(s), want one per model (%d)", got, want)
	}
	if got := r.count("section"); got != 0 {
		t.Errorf("%d section call(s) for a bill that fits, want 0", got)
	}

	for _, m := range models.Catalog {
		memo, err := st.Deliberation(ctx, bill.ID, m.Key)
		if err != nil {
			t.Fatalf("Deliberation(%s): %v", m.Key, err)
		}
		if len(memo.Pros) == 0 || len(memo.Cons) == 0 {
			t.Errorf("%s memo is empty: %+v", m.Key, memo)
		}
		if memo.TextHash != bill.TextFingerprint() {
			t.Errorf("%s memo is not tied to the text it read", m.Key)
		}
	}
	missing, err := st.MissingVoteModels(ctx, bill.ID)
	if err != nil {
		t.Fatalf("MissingVoteModels: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("%d model(s) still missing a verdict", len(missing))
	}
}

// A re-vote reads the memo the model already wrote rather than improvising a
// new one.
func TestReVoteReusesTheStoredMemo(t *testing.T) {
	ctx := context.Background()
	svc, st, r := newService(t)
	bill := testBill()
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	if err := svc.VoteBill(ctx, bill.ID, false); err != nil {
		t.Fatalf("VoteBill: %v", err)
	}
	memoCalls := r.count("memo")

	if err := svc.VoteBill(ctx, bill.ID, true); err != nil {
		t.Fatalf("VoteBill(force): %v", err)
	}
	if got := r.count("memo"); got != memoCalls {
		t.Errorf("memo calls grew from %d to %d on a re-vote", memoCalls, got)
	}
	if got, want := r.count("vote"), 2*len(models.Catalog); got != want {
		t.Errorf("%d vote call(s) after a forced re-vote, want %d", got, want)
	}
}

// A memo written against superseded text is not reused: the model reads the new
// text and writes a new one.
func TestNewStatuteTextForcesAFreshMemo(t *testing.T) {
	ctx := context.Background()
	svc, st, r := newService(t)
	bill := testBill()
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	if err := st.SaveDeliberation(ctx, models.Deliberation{
		BillID:   bill.ID,
		ModelKey: models.Catalog[0].Key,
		Pros:     []string{"Written against the introduced print."},
		Cons:     []string{"Which is no longer the text on the floor."},
		TextHash: models.Fingerprint("an earlier print of this bill"),
	}); err != nil {
		t.Fatalf("SaveDeliberation: %v", err)
	}

	if err := svc.VoteBill(ctx, bill.ID, false); err != nil {
		t.Fatalf("VoteBill: %v", err)
	}
	if got, want := r.count("memo"), len(models.Catalog); got != want {
		t.Errorf("%d memo call(s), want %d: the stale memo should have been rewritten", got, want)
	}
	memo, err := st.Deliberation(ctx, bill.ID, models.Catalog[0].Key)
	if err != nil {
		t.Fatalf("Deliberation: %v", err)
	}
	if memo.TextHash != bill.TextFingerprint() {
		t.Errorf("memo hash = %q, want the current text", memo.TextHash)
	}
	if strings.Contains(strings.Join(memo.Pros, " "), "introduced print") {
		t.Error("the stale memo survived")
	}
}

// A bill with no statute text is not voted on at all. The summary is not a
// substitute for the law.
func TestVoteBillRefusesABillWithoutStatuteText(t *testing.T) {
	ctx := context.Background()
	svc, st, r := newService(t)
	bill := testBill()
	bill.FullText = ""
	bill.TextSource = ""
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}

	if err := svc.VoteBill(ctx, bill.ID, false); err == nil {
		t.Fatal("expected VoteBill to refuse a bill with no statute text")
	}
	if got := r.count("memo") + r.count("vote"); got != 0 {
		t.Errorf("made %d model call(s) for a bill with nothing to read", got)
	}
}

// Bootstrap on an empty database loads the offline corpus, which is statute
// text, and votes it.
func TestBootstrapVotesTheSeedCorpus(t *testing.T) {
	ctx := context.Background()
	svc, st, _ := newService(t)

	if err := svc.Bootstrap(ctx, 30*time.Second); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	// The backfill runs in the background; the featured bill is voted before
	// Bootstrap returns.
	bills, err := st.Bills(ctx)
	if err != nil {
		t.Fatalf("Bills: %v", err)
	}
	if len(bills) == 0 {
		t.Fatal("no bills were loaded")
	}
	for _, b := range bills {
		if !b.HasStatuteText() {
			t.Errorf("%s was loaded without statute text", b.ID)
		}
	}
	missing, err := st.MissingVoteModels(ctx, bills[0].ID)
	if err != nil {
		t.Fatalf("MissingVoteModels: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("the featured bill still needs %d verdict(s) after bootstrap", len(missing))
	}
	if svc.Status().Source != models.SourceSeed {
		t.Errorf("source = %q, want the seed corpus", svc.Status().Source)
	}
}
