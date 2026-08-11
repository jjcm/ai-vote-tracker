package openrouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

func serverReturning(t *testing.T, content string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q, want the bearer key", got)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "anthropic/claude-opus-5" {
			t.Errorf("model = %v", body["model"])
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": content}}},
		})
	}))
}

func testBill() models.Bill {
	return models.Bill{
		Number:  "S. 1264",
		Title:   "Border Security and Asylum Reform Act of 2025",
		Chamber: models.ChamberSenate,
		Summary: "A bill about border security.",
	}
}

func TestVoteParsesPlainJSON(t *testing.T) {
	srv := serverReturning(t, `{"vote":"Yes","reason":"Strengthens security while preserving due process."}`)
	defer srv.Close()

	c := New(srv.URL, "test-key", "", 5*time.Second)
	got, err := c.Vote(context.Background(), models.Catalog[0], testBill())
	if err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if got.Vote != models.VoteYes {
		t.Errorf("vote = %q, want Yes", got.Vote)
	}
	if got.Reason != "Strengthens security while preserving due process." {
		t.Errorf("reason = %q", got.Reason)
	}
}

func TestVoteParsesFencedJSONAndLooseCasing(t *testing.T) {
	srv := serverReturning(t, "Sure!\n```json\n{\"vote\": \"NAY\", \"reason\": \"Too broad.\"}\n```")
	defer srv.Close()

	c := New(srv.URL, "test-key", "", 5*time.Second)
	got, err := c.Vote(context.Background(), models.Catalog[0], testBill())
	if err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if got.Vote != models.VoteNo {
		t.Errorf("vote = %q, want No", got.Vote)
	}
}

func TestVoteTrimsRationaleToOneSentence(t *testing.T) {
	srv := serverReturning(t, `{"vote":"Yes","reason":"First sentence here. Second sentence should be dropped."}`)
	defer srv.Close()

	c := New(srv.URL, "test-key", "", 5*time.Second)
	got, _ := c.Vote(context.Background(), models.Catalog[0], testBill())
	if got.Reason != "First sentence here." {
		t.Errorf("reason = %q, want only the first sentence", got.Reason)
	}
}

func TestVoteRejectsUnusableVerdict(t *testing.T) {
	srv := serverReturning(t, `{"vote":"maybe","reason":"It depends."}`)
	defer srv.Close()

	c := New(srv.URL, "test-key", "", 5*time.Second)
	if _, err := c.Vote(context.Background(), models.Catalog[0], testBill()); err == nil {
		t.Fatal("expected an error for a non-binary verdict")
	}
}

// The exact reply that leaked into the S. 1264 page: Gemini stopped mid-string
// when the token budget ran out, and the prose fallback published the envelope
// as the rationale.
func TestVoteRecoversFieldsFromTruncatedJSON(t *testing.T) {
	const truncated = `{"vote": "Yes", "reason": "This.`

	got, err := parseVerdict(truncated)
	if err != nil {
		t.Fatalf("parseVerdict: %v", err)
	}
	if got.Vote != "Yes" {
		t.Errorf("vote = %q, want Yes", got.Vote)
	}
	if got.Reason != "This." {
		t.Errorf("reason = %q, want the recovered string only", got.Reason)
	}
	assertNoScaffolding(t, got.Reason)
}

// The same reply arriving over the wire must not be stored either: it is a
// fragment, so the call fails and the next round retries it.
func TestTruncatedFragmentIsRejectedRatherThanStored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]string{"content": `{"vote": "Yes", "reason": "This.`},
				"finish_reason": "length",
			}},
		})
	}))
	defer srv.Close()

	_, err := New(srv.URL, "test-key", "", 5*time.Second).Vote(context.Background(), models.Catalog[0], testBill())
	if err == nil {
		t.Fatal("a rationale cut off after one word should fail the call")
	}
	if !strings.Contains(err.Error(), "ran out of tokens") {
		t.Errorf("err = %v, want it to name the cause", err)
	}
}

// A reply cut short is retried with a larger budget before it is given up on.
func TestTruncatedReplyIsRetriedWithMoreRoom(t *testing.T) {
	var budgets []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			MaxTokens int `json:"max_tokens"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		budgets = append(budgets, body.MaxTokens)

		content, finish := `{"vote": "No", "reason": "Prea`, "length"
		if len(budgets) > 1 {
			content, finish = `{"vote":"No","reason":"Preempts state experimentation before the federal approach is proven."}`, "stop"
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]string{"content": content},
				"finish_reason": finish,
			}},
		})
	}))
	defer srv.Close()

	got, err := New(srv.URL, "test-key", "", 5*time.Second).Vote(context.Background(), models.Catalog[0], testBill())
	if err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if got.Reason != "Preempts state experimentation before the federal approach is proven." {
		t.Errorf("reason = %q, want the complete second attempt", got.Reason)
	}
	if len(budgets) != 2 {
		t.Fatalf("made %d attempts, want 2", len(budgets))
	}
	if budgets[1] <= budgets[0] {
		t.Errorf("retry budget %d did not grow from %d", budgets[1], budgets[0])
	}
}

// Every shape a model has actually produced must yield a bare sentence.
func TestReasonNeverCarriesJSONScaffolding(t *testing.T) {
	sentence := "Balances national security with the reforms needed to reduce backlogs."
	cases := map[string]struct{ reply, want string }{
		"plain":              {`{"vote":"Yes","reason":"` + sentence + `"}`, sentence},
		"fenced":             {"```json\n{\"vote\":\"Yes\",\"reason\":\"" + sentence + "\"}\n```", sentence},
		"chatter around":     {"Here is my vote:\n{\"vote\":\"Yes\",\"reason\":\"" + sentence + "\"}\nHope that helps.", sentence},
		"truncated envelope": {`{"vote": "Yes", "reason": "` + sentence, sentence},
		"missing brace":      {`{"vote": "Yes", "reason": "` + sentence + `"`, sentence},
		"escaped quotes":     {`{"vote":"Yes","reason":"Backs the \"expedited\" track."}`, `Backs the "expedited" track.`},
		"double wrapped":     {`{"vote":"Yes","reason":"{\"vote\": \"Yes\", \"reason\": \"` + sentence + `\"}"}`, sentence},
		// Braces inside the rationale are content, not scaffolding.
		"nested braces": {`{"vote":"Yes","reason":"Funds the program {see sec. 4} in full."}`, "Funds the program {see sec. 4} in full."},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := parseVerdict(tc.reply)
			if err != nil {
				t.Fatalf("parseVerdict: %v", err)
			}
			if models.NormalizeVote(got.Vote) != models.VoteYes {
				t.Errorf("vote = %q, want Yes", got.Vote)
			}
			if got.Reason != tc.want {
				t.Errorf("reason = %q, want %q", got.Reason, tc.want)
			}
			assertNoScaffolding(t, got.Reason)
		})
	}
}

// assertNoScaffolding checks for the response envelope specifically, not for
// braces, which a rationale is free to contain.
func assertNoScaffolding(t *testing.T, reason string) {
	t.Helper()
	for _, leak := range []string{`"vote"`, `"reason"`, "```"} {
		if strings.Contains(reason, leak) {
			t.Errorf("reason %q still contains %q", reason, leak)
		}
	}
	if strings.HasPrefix(reason, "{") {
		t.Errorf("reason %q starts with the response envelope", reason)
	}
}

func TestProseReplyIsSalvagedWithoutWordFragments(t *testing.T) {
	got, err := parseVerdict("Nonetheless, I would vote Yes because the funding is real.")
	if err != nil {
		t.Fatalf("parseVerdict: %v", err)
	}
	if got.Vote != models.VoteYes {
		t.Errorf("vote = %q, want Yes: 'Nonetheless' must not read as a No", got.Vote)
	}
}

func TestFirstSentenceKeepsInitialisms(t *testing.T) {
	cases := map[string]string{
		"Strengthens U.S. border security. Second sentence.": "Strengthens U.S. border security.",
		"Costs about 1.5 percent of outlays. And more.":      "Costs about 1.5 percent of outlays.",
		"Cited by Sen. Bennet in committee. Next.":           "Cited by Sen. Bennet in committee.",
		"No trailing punctuation":                            "No trailing punctuation.",
	}
	for in, want := range cases {
		if got := firstSentence(in); got != want {
			t.Errorf("firstSentence(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVoteWithoutKeyFails(t *testing.T) {
	c := New("https://openrouter.ai/api/v1", "", "", time.Second)
	if _, err := c.Vote(context.Background(), models.Catalog[0], testBill()); err != ErrNoKey {
		t.Errorf("err = %v, want ErrNoKey", err)
	}
}

func TestScoreIdeologyClampsToRange(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": `{"score": 4.2}`}}},
		})
	}))
	defer srv.Close()

	c := New(srv.URL, "test-key", "", 5*time.Second)
	score, err := c.ScoreIdeology(context.Background(), testBill())
	if err != nil {
		t.Fatalf("ScoreIdeology: %v", err)
	}
	if score != 1 {
		t.Errorf("score = %v, want it clamped to 1", score)
	}
}

func TestBillTextReachesThePrompt(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []struct{ Content string } `json:"messages"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		seen = body.Messages[len(body.Messages)-1].Content
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]string{"content": `{"vote":"No","reason":"x"}`}}},
		})
	}))
	defer srv.Close()

	b := testBill()
	b.FullText = "SEC. 4. EXPEDITED REMOVAL."
	New(srv.URL, "k", "", 5*time.Second).Vote(context.Background(), models.Catalog[0], b)

	for _, want := range []string{"S. 1264", b.Title, "SEC. 4. EXPEDITED REMOVAL."} {
		if !strings.Contains(seen, want) {
			t.Errorf("prompt is missing %q", want)
		}
	}
}
