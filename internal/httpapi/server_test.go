package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/congress"
	"github.com/pwnies/ai-vote-tracker/internal/models"
	"github.com/pwnies/ai-vote-tracker/internal/openrouter"
	"github.com/pwnies/ai-vote-tracker/internal/store"
	"github.com/pwnies/ai-vote-tracker/internal/votes"
)

// A model call takes this long in these tests, standing in for the minutes a
// real deliberation over a large bill takes.
const providerDelay = 3 * time.Second

// newServer wires the real handler onto a real store and a stand-in for
// OpenRouter that answers no faster than providerDelay.
func newServer(t *testing.T) (http.Handler, *store.Store) {
	return newSlowServer(t, providerDelay)
}

// newSlowServer is newServer with the provider's latency chosen by the caller,
// for tests whose cleanup has to wait out a whole corpus.
func newSlowServer(t *testing.T, delay time.Duration) (http.Handler, *store.Store) {
	t.Helper()

	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	// Cleanups run last-registered-first, and the order matters here: the round
	// these tests queue is still running when the test body ends, so it has to
	// be drained before the provider goes away and the database closes under it.
	t.Cleanup(func() { st.Close() })

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delay)
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		var system string
		for _, m := range body.Messages {
			if m.Role == "system" {
				system = m.Content
			}
		}
		content := `{"vote": "Yes", "reason": "Funds the program with workable deadlines."}`
		if strings.Contains(system, "pros and cons memo") && !strings.Contains(system, "recorded floor vote") {
			content = `{"pros": ["Sec. 2 funds it."], "cons": ["Sec. 5 is vague."]}`
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
		})
	}))
	t.Cleanup(provider.Close)

	logger := log.New(io.Discard, "", 0)
	client := openrouter.New(provider.URL, "test-key", "", time.Minute)
	svc := votes.New(st, client, congress.New("", ""), 4, logger)
	t.Cleanup(func() { drain(t, svc) })

	web := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
		"bills.html": &fstest.MapFile{Data: []byte("<bills-browser></bills-browser>")},
		"bill.html":  &fstest.MapFile{Data: []byte("<bill-detail></bill-detail>")},
		"404.html":   &fstest.MapFile{Data: []byte("<html>not found</html>")},
	}
	return New(st, svc, web, logger).Handler(), st
}

// drain waits for background rounds to finish so they are not still writing
// when the test's database closes.
func drain(t *testing.T, svc *votes.Service) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if status := svc.Status(); status.BillsInFlight == 0 && !status.Backfilling {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Error("timed out waiting for background rounds to finish")
}

func seedBill(t *testing.T, st *store.Store) models.Bill {
	t.Helper()
	bill := models.Bill{
		ID:         "s-1264",
		Number:     "S. 1264",
		Title:      "Border Security and Asylum Reform Act of 2025",
		Chamber:    models.ChamberSenate,
		Status:     "Read twice and referred to committee",
		Summary:    "A CRS summary, for the bill card only.",
		FullText:   `<section><enum>1.</enum><header>Short title</header><text>The Testing Act.</text></section>`,
		TextSource: models.TextSourceCongressXML,
		UpdatedAt:  time.Now().UTC(),
	}
	if _, err := st.UpsertBill(context.Background(), bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}
	return bill
}

func do(t *testing.T, h http.Handler, method, target string) (*httptest.ResponseRecorder, map[string]any, time.Duration) {
	t.Helper()
	rec := httptest.NewRecorder()
	start := time.Now()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	elapsed := time.Since(start)

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("%s %s returned %d with a non-JSON body: %s", method, target, rec.Code, rec.Body.String())
	}
	return rec, payload, elapsed
}

// Asking for a round queues one and answers immediately. A deliberation runs
// for minutes; no request is held open for it.
func TestVoteEndpointQueuesRatherThanWaits(t *testing.T) {
	h, st := newServer(t)
	bill := seedBill(t, st)

	rec, payload, elapsed := do(t, h, http.MethodPost, "/api/bills/"+bill.ID+"/vote")
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if elapsed >= providerDelay {
		t.Errorf("the handler blocked for %s on a provider answering in %s", elapsed, providerDelay)
	}
	if payload["voting"] != true || payload["queued"] != true {
		t.Errorf("payload = %v, want it to report a queued round", payload)
	}
	// The reply carries the cached state, which at this point is all pending.
	got := payload["bill"].(map[string]any)["votes"].([]any)
	if len(got) != len(models.Catalog) {
		t.Fatalf("%d verdict(s) in the reply, want one column per model", len(got))
	}
	for _, v := range got {
		if vote := v.(map[string]any)["vote"]; vote != models.VotePending {
			t.Errorf("verdict = %v, want the cached Pending state", vote)
		}
	}

	// A second request joins the round in flight instead of starting another.
	waitForRound(t, h)
	_, second, _ := do(t, h, http.MethodPost, "/api/bills/"+bill.ID+"/vote")
	if second["queued"] != false {
		t.Errorf("payload = %v, want the second request to report joining the round", second)
	}
}

// While that round runs, the read endpoints keep serving from the cache.
func TestReadEndpointsServeCachedStateDuringARound(t *testing.T) {
	h, st := newServer(t)
	bill := seedBill(t, st)

	if rec, _, _ := do(t, h, http.MethodPost, "/api/bills/"+bill.ID+"/vote"); rec.Code != http.StatusAccepted {
		t.Fatalf("queueing the round returned %d", rec.Code)
	}
	waitForRound(t, h)

	for _, target := range []string{"/api/featured", "/api/bills", "/api/bills/" + bill.ID, "/api/status"} {
		rec, payload, elapsed := do(t, h, http.MethodGet, target)
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s returned %d", target, rec.Code)
		}
		if elapsed >= providerDelay {
			t.Errorf("GET %s blocked for %s while a round was running", target, elapsed)
		}
		if payload == nil {
			t.Errorf("GET %s returned no payload", target)
		}
	}

}

// waitForRound blocks until the pipeline reports a round in flight, which the
// page polls on and which the tests need before asserting anything about it.
func waitForRound(t *testing.T, h http.Handler) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		_, status, _ := do(t, h, http.MethodGet, "/api/status")
		if status["billsInFlight"] != float64(0) || status["backfilling"] == true {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the queued round to appear in the pipeline status")
}

// Refreshing reads a dozen bills' text from Congress.gov and then votes them
// all, which is minutes of work. The handler starts it and answers.
func TestRefreshEndpointReturnsImmediately(t *testing.T) {
	// A brisk provider here: this test's cleanup waits out a refresh of the
	// whole corpus, which is 13 bills at two calls per model.
	h, _ := newSlowServer(t, 100*time.Millisecond)

	rec, payload, elapsed := do(t, h, http.MethodPost, "/api/refresh")
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", rec.Code)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("the handler blocked for %s on a refresh that takes seconds", elapsed)
	}
	if payload["refreshing"] != true {
		t.Errorf("payload = %v", payload)
	}

	_, second, _ := do(t, h, http.MethodPost, "/api/refresh")
	if second["alreadyRunning"] != true {
		t.Errorf("payload = %v, want the second request to report the refresh already running", second)
	}
}

func TestVoteEndpointOnAMissingBill(t *testing.T) {
	h, _ := newServer(t)
	rec, _, _ := do(t, h, http.MethodPost, "/api/bills/nope/vote")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// Congress.gov's CRS summaries open by repeating the bill's short title, which
// the cards already print above them. Rows stored by an earlier build still
// carry that echo, so the API strips it on the way out.
func TestSummariesDoNotEchoTheBillTitle(t *testing.T) {
	h, st := newServer(t)
	bill := seedBill(t, st)
	bill.Summary = bill.Title + " This bill overhauls border security and rewrites the asylum process."
	if _, err := st.UpsertBill(context.Background(), bill); err != nil {
		t.Fatalf("UpsertBill: %v", err)
	}

	const want = "This bill overhauls border security and rewrites the asylum process."
	for _, target := range []string{"/api/bills/" + bill.ID, "/api/featured", "/api/bills", "/api/alignment"} {
		_, payload, _ := do(t, h, http.MethodGet, target)
		for _, got := range summaries(t, payload, bill.ID) {
			if got != want {
				t.Errorf("GET %s served summary %q, want the echo stripped: %q", target, got, want)
			}
		}
	}
}

// summaries pulls every copy of one bill's summary out of a response, whichever
// key that response files bills under.
func summaries(t *testing.T, payload map[string]any, billID string) []string {
	t.Helper()
	var out []string
	collect := func(v any) {
		bill, ok := v.(map[string]any)
		if ok && bill["id"] == billID {
			out = append(out, bill["summary"].(string))
		}
	}
	for _, key := range []string{"bill", "featured"} {
		if v, ok := payload[key]; ok {
			collect(v)
		}
	}
	for _, key := range []string{"bills", "latest", "recentBills"} {
		list, ok := payload[key].([]any)
		if !ok {
			continue
		}
		for _, v := range list {
			collect(v)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no copy of %s found in the response", billID)
	}
	return out
}

// Clicking a row goes to /bills/{id}, which is one client-rendered document
// served for every bill, whether or not that id exists.
func TestBillDetailPageIsServedForAnyBillPath(t *testing.T) {
	h, st := newServer(t)
	bill := seedBill(t, st)

	for _, target := range []string{"/bills/" + bill.ID, "/bills/does-not-exist"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s returned %d, want 200", target, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "<bill-detail>") {
			t.Errorf("GET %s served %q, want the bill detail page", target, rec.Body.String())
		}
	}

	// The listing itself is untouched.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/bills", nil))
	if !strings.Contains(rec.Body.String(), "<bills-browser>") {
		t.Errorf("GET /bills served %q, want the listing page", rec.Body.String())
	}
}

// The statute text is opt-in: a live bill's XML is megabytes, and the page does
// not read it.
func TestBillDetailWithholdsTheStatuteTextUnlessAsked(t *testing.T) {
	h, st := newServer(t)
	bill := seedBill(t, st)

	_, payload, _ := do(t, h, http.MethodGet, "/api/bills/"+bill.ID)
	if text := payload["bill"].(map[string]any)["fullText"]; text != nil {
		t.Errorf("fullText = %v, want it withheld by default", text)
	}
	if chars := payload["textChars"]; chars != float64(len(bill.FullText)) {
		t.Errorf("textChars = %v, want %d", chars, len(bill.FullText))
	}

	_, asked, _ := do(t, h, http.MethodGet, "/api/bills/"+bill.ID+"?text=true")
	if text := asked["bill"].(map[string]any)["fullText"]; text != bill.FullText {
		t.Errorf("fullText = %v, want the statute text", text)
	}
}

// The alignment payload carries the 2028 comparison the page renders, and the
// potentials who are floated for the office but hold no seat.
func TestAlignmentCarriesContenderMatches(t *testing.T) {
	ctx := context.Background()
	h, st := newServer(t)

	for i, id := range []string{"hr-1-119", "hr-2-119", "hr-3-119"} {
		bill := models.Bill{ID: id, Number: strings.ToUpper(id), Title: "A tracked bill",
			Chamber: models.ChamberHouse, FullText: "<section/>", TextSource: models.TextSourceCongressXML,
			UpdatedAt: time.Date(2026, 7, i+1, 0, 0, 0, 0, time.UTC)}
		if _, err := st.UpsertBill(ctx, bill); err != nil {
			t.Fatalf("UpsertBill: %v", err)
		}
		if err := st.SaveVote(ctx, models.Vote{BillID: id, ModelKey: "opus", Vote: models.VoteYes,
			CreatedAt: time.Unix(int64(1700+i), 0)}); err != nil {
			t.Fatalf("SaveVote: %v", err)
		}
		// Ocasio-Cortez is on the record for all three and splits on the last.
		position := models.PositionYea
		if i == 2 {
			position = models.PositionNay
		}
		if err := st.SaveMemberVote(ctx, models.MemberVote{BillID: id, Bioguide: "O000172",
			Chamber: models.ChamberHouse, Position: position,
			VotedAt: time.Date(2026, 7, i+1, 0, 0, 0, 0, time.UTC)}); err != nil {
			t.Fatalf("SaveMemberVote: %v", err)
		}
	}

	rec, payload, _ := do(t, h, http.MethodGet, "/api/alignment")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	matches, ok := payload["contenderMatches"].(map[string]any)
	if !ok {
		t.Fatalf("the alignment payload carries no contenderMatches: %v", payload)
	}
	if got := matches["minOverlap"]; got != float64(3) {
		t.Errorf("minOverlap = %v, want the default of 3", got)
	}
	if got := matches["memberPositions"]; got != float64(3) {
		t.Errorf("memberPositions = %v, want 3", got)
	}
	if outsiders, _ := matches["notInCongress"].([]any); len(outsiders) == 0 {
		t.Error("the potentials with no seat should be named, so the page can say why they carry no score")
	}

	closest := closestFor(t, matches, "opus")
	if closest["name"] != "Alexandria Ocasio-Cortez" {
		t.Errorf("closest = %v", closest["name"])
	}
	if closest["overlap"] != float64(3) || closest["agreed"] != float64(2) {
		t.Errorf("closest agreed on %v of %v, want 2 of 3", closest["agreed"], closest["overlap"])
	}

	// The second request is served from the table the first one cached, and
	// has to say the same thing.
	_, repeat, _ := do(t, h, http.MethodGet, "/api/alignment")
	again := closestFor(t, repeat["contenderMatches"].(map[string]any), "opus")
	if again["name"] != closest["name"] || again["agreement"] != closest["agreement"] {
		t.Errorf("the cached table disagrees with the one that was computed: %v vs %v", again, closest)
	}
}

// closestFor digs one model's best-matching member out of the JSON payload.
func closestFor(t *testing.T, matches map[string]any, modelKey string) map[string]any {
	t.Helper()
	models, _ := matches["models"].([]any)
	for _, entry := range models {
		m, _ := entry.(map[string]any)
		if m["key"] != modelKey {
			continue
		}
		closest, ok := m["closest"].(map[string]any)
		if !ok {
			t.Fatalf("%s has no closest match: %v", modelKey, m)
		}
		return closest
	}
	t.Fatalf("no ranking for %s in %v", modelKey, matches)
	return nil
}
