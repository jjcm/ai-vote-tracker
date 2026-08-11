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
