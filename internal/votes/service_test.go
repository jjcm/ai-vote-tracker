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
// the tests can tell a memo that was written from one that was reused. It can
// also be made slow, or made to fail, to stand in for a real provider.
type router struct {
	delay time.Duration // set before the service is used

	mu        sync.Mutex
	stages    map[string]int
	failVotes int
}

func (r *router) count(stage string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.stages[stage]
}

// failVotesUntil makes the next n vote calls fail, the way a rate limit does.
func (r *router) failVotesUntil(n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failVotes = n
}

// observe records a call and reports whether it should fail.
func (r *router) observe(stage string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.stages[stage]++
	if stage == "vote" && r.failVotes > 0 {
		r.failVotes--
		return true
	}
	return false
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

		if r.delay > 0 {
			time.Sleep(r.delay)
		}
		// A 400 is not retried inside the client, so an injected failure lands
		// as a recorded Error and it is the backfill that has to come back for
		// it.
		if r.observe(stage) {
			http.Error(w, `{"error":{"message":"the provider refused this request"}}`, http.StatusBadRequest)
			return
		}

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
	// Production waits minutes between backfill sweeps because nothing is
	// waiting on the result. A test is.
	svc.retryDelay = 10 * time.Millisecond
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

// Filling in a verdict that failed earlier reads the memo that model already
// wrote for this text rather than improvising a new one.
func TestFillingInAVerdictReusesTheStoredMemo(t *testing.T) {
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

	// One model's verdict goes missing, the way a rate limited call leaves it.
	if err := st.SaveVote(ctx, models.Vote{
		BillID: bill.ID, ModelKey: models.Catalog[1].Key, Vote: models.VoteError, Error: "rate limited",
	}); err != nil {
		t.Fatalf("SaveVote: %v", err)
	}
	if err := svc.VoteBill(ctx, bill.ID, false); err != nil {
		t.Fatalf("VoteBill: %v", err)
	}

	if got := r.count("memo"); got != memoCalls {
		t.Errorf("memo calls grew from %d to %d while filling in one verdict", memoCalls, got)
	}
	if got, want := r.count("vote"), len(models.Catalog)+1; got != want {
		t.Errorf("%d vote call(s), want %d: only the missing verdict should be re-collected", got, want)
	}
}

// Asking for a re-run means a re-run: the model reads the bill again and writes
// a new memo rather than voting off the old one.
func TestForcedRoundRewritesTheMemo(t *testing.T) {
	ctx := context.Background()
	svc, st, r := newService(t)
	bill := testBill()
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	if err := svc.VoteBill(ctx, bill.ID, false); err != nil {
		t.Fatalf("VoteBill: %v", err)
	}

	if err := svc.VoteBill(ctx, bill.ID, true); err != nil {
		t.Fatalf("VoteBill(force): %v", err)
	}
	if got, want := r.count("memo"), 2*len(models.Catalog); got != want {
		t.Errorf("%d memo call(s) after a forced round, want %d", got, want)
	}
	if got, want := r.count("vote"), 2*len(models.Catalog); got != want {
		t.Errorf("%d vote call(s) after a forced round, want %d", got, want)
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

// Bootstrap loads the corpus and hands the verdicts to the background backfill,
// which works through every bill and leaves a memo behind each verdict.
func TestBootstrapLoadsTheCorpusAndVotesItInTheBackground(t *testing.T) {
	ctx := context.Background()
	svc, st, _ := newService(t)

	if err := svc.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
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
	if svc.Status().Source != models.SourceSeed {
		t.Errorf("source = %q, want the seed corpus", svc.Status().Source)
	}

	waitUntil(t, 60*time.Second, "every bill to have a full set of verdicts", func() bool {
		pending, err := st.BillIDsMissingVotes(ctx)
		return err == nil && len(pending) == 0
	})
	waitUntil(t, 30*time.Second, "the backfill to report itself finished", func() bool {
		return !svc.Status().Backfilling
	})

	for _, b := range bills {
		for _, m := range models.Catalog {
			if _, err := st.Deliberation(ctx, b.ID, m.Key); err != nil {
				t.Errorf("%s/%s has a verdict but no memo behind it: %v", b.ID, m.Key, err)
			}
		}
	}
}

// Startup must not wait on a model. The site comes up, serves what is cached,
// and shows the rest as pending while the backfill runs behind it.
func TestBootstrapDoesNotWaitOnAModel(t *testing.T) {
	ctx := context.Background()
	svc, st, r := newService(t)
	// A bill already in the database, so this measures the wait for verdicts
	// rather than the wait for a corpus.
	if _, err := st.UpsertBill(ctx, testBill()); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	r.delay = 2 * time.Second

	start := time.Now()
	if err := svc.Bootstrap(ctx); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if elapsed := time.Since(start); elapsed > r.delay {
		t.Errorf("Bootstrap blocked for %s on a provider answering in %s", elapsed, r.delay)
	}
	// Returning fast is only right if the work was handed off rather than
	// dropped, so the round has to show up behind us.
	waitUntil(t, 10*time.Second, "the backfill to pick the round up", func() bool {
		return svc.Status().Backfilling || r.count("memo") > 0
	})

	waitUntil(t, 60*time.Second, "the backfill to finish", func() bool {
		return !svc.Status().Backfilling
	})
	if missing, err := st.MissingVoteModels(ctx, testBill().ID); err != nil || len(missing) != 0 {
		t.Errorf("%d verdict(s) missing after the backfill (err %v)", len(missing), err)
	}
}

// A bill whose verdicts failed on the first sweep gets another one, rather than
// showing an error until somebody notices.
func TestBackfillSweepsAgainAfterAFailure(t *testing.T) {
	ctx := context.Background()
	svc, st, r := newService(t)
	if _, err := st.UpsertBill(ctx, testBill()); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}

	// The first vote call of the run fails outright; a second sweep gets a
	// verdict for the model it cost.
	r.failVotesUntil(1)
	svc.backfill(ctx)

	missing, err := st.MissingVoteModels(ctx, testBill().ID)
	if err != nil {
		t.Fatalf("MissingVoteModels: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("%d verdict(s) still missing after the retry sweeps", len(missing))
	}
	if r.count("vote") <= len(models.Catalog) {
		t.Errorf("%d vote call(s): the failed verdict was never retried", r.count("vote"))
	}
}

// StartRound is what the HTTP layer calls, and it has to return at once.
func TestStartRoundReturnsBeforeTheRoundDoes(t *testing.T) {
	ctx := context.Background()
	svc, st, r := newService(t)
	r.delay = 300 * time.Millisecond
	bill := testBill()
	if _, err := st.UpsertBill(ctx, bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}

	start := time.Now()
	if !svc.StartRound(bill.ID, false) {
		t.Fatal("StartRound did not start a round")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("StartRound blocked for %s", elapsed)
	}

	waitUntil(t, 30*time.Second, "the queued round to finish", func() bool {
		pending, err := st.MissingVoteModels(ctx, bill.ID)
		return err == nil && len(pending) == 0
	})
}

// Refresh is called from a request handler, so it may not block either.
func TestRefreshReturnsImmediately(t *testing.T) {
	svc, st, r := newService(t)
	r.delay = 200 * time.Millisecond

	start := time.Now()
	if !svc.Refresh() {
		t.Fatal("Refresh did not start")
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("Refresh blocked for %s", elapsed)
	}
	if svc.Refresh() {
		t.Error("a second Refresh should have found the first one running")
	}

	waitUntil(t, 60*time.Second, "the refresh to load and vote the corpus", func() bool {
		n, err := st.CountBills(context.Background())
		if err != nil || n == 0 {
			return false
		}
		pending, err := st.BillIDsMissingVotes(context.Background())
		return err == nil && len(pending) == 0
	})
}

// waitUntil polls a condition so a test can wait for background work without
// leaving goroutines running into cleanup.
func waitUntil(t *testing.T, limit time.Duration, what string, done func() bool) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if done() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out after %s waiting for %s", limit, what)
}
