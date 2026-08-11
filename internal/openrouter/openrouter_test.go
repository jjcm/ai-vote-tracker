package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// call is one request the client made, as the provider saw it.
type call struct {
	model     string
	system    string
	user      string
	maxTokens int
}

// stage names the pipeline step a call belongs to, read off the system prompt
// the same way the development mock router reads it.
func (c call) stage() string {
	switch {
	case strings.Contains(c.system, "SECTION NOTE"):
		return "section"
	// Checked before the memo: the vote prompt refers to the memo the model
	// wrote, so the memo marker appears in both.
	case strings.Contains(c.system, "recorded floor vote"):
		return "vote"
	case strings.Contains(c.system, "pros and cons memo"):
		return "memo"
	case strings.Contains(c.system, "ideological direction"):
		return "ideology"
	}
	return "unknown"
}

type fakeRouter struct {
	mu    sync.Mutex
	calls []call
	reply func(call) (content string, finish string)
}

func (f *fakeRouter) record(c call) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
}

func (f *fakeRouter) seen(stage string) []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if c.stage() == stage {
			out = append(out, c)
		}
	}
	return out
}

// defaultReply answers every pipeline stage with well-formed JSON.
func defaultReply(c call) (string, string) {
	switch c.stage() {
	case "section":
		return `{"note": "This part authorizes appropriations and sets a deadline."}`, "stop"
	case "memo":
		return `{"pros": ["Sec. 2 funds the program in full."], "cons": ["Sec. 5 leaves the standard undefined."]}`, "stop"
	case "vote":
		return `{"vote": "Yes", "reason": "Funds the program in full with workable deadlines."}`, "stop"
	case "ideology":
		return `{"score": 0.4}`, "stop"
	}
	return `{}`, "stop"
}

func newRouter(t *testing.T, reply func(call) (string, string)) (*Client, *fakeRouter) {
	t.Helper()
	if reply == nil {
		reply = defaultReply
	}
	fake := &fakeRouter{reply: reply}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			Messages  []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		c := call{model: body.Model, maxTokens: body.MaxTokens}
		for _, m := range body.Messages {
			switch m.Role {
			case "system":
				c.system = m.Content
			case "user":
				c.user = m.Content
			}
		}
		fake.record(c)

		content, finish := fake.reply(c)
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"message":       map[string]string{"content": content},
				"finish_reason": finish,
			}},
		})
	}))
	t.Cleanup(srv.Close)
	return New(srv.URL, "test-key", "", 5*time.Second), fake
}

func testBill() models.Bill {
	return models.Bill{
		ID:      "s-1264",
		Number:  "S. 1264",
		Title:   "Border Security and Asylum Reform Act of 2025",
		Chamber: models.ChamberSenate,
		// The CRS summary exists for the bill cards and must never reach a
		// model.
		Summary:    "A CRS summary of the border security bill.",
		FullText:   `<section id="H1"><enum>4.</enum><header>Expedited removal</header><text>Section 235(b)(1) is amended.</text></section>`,
		TextSource: models.TextSourceCongressXML,
	}
}

func testMemo() models.Deliberation {
	return models.Deliberation{
		BillID:   "s-1264",
		ModelKey: models.Catalog[0].Key,
		Pros:     []string{"Sec. 3 funds 3,000 additional agents."},
		Cons:     []string{"Sec. 5 raises the credible fear standard sharply."},
	}
}

func deliberateAndVote(t *testing.T, c *Client, bill models.Bill) Verdict {
	t.Helper()
	m := models.Catalog[0]
	memo, err := c.Deliberate(context.Background(), m, bill)
	if err != nil {
		t.Fatalf("Deliberate: %v", err)
	}
	verdict, err := c.Vote(context.Background(), m, bill, memo)
	if err != nil {
		t.Fatalf("Vote: %v", err)
	}
	return verdict
}

// The pipeline has to run in order — memo first, then the vote — and the vote
// has to rest on the memo the same model just wrote.
func TestEveryBillGetsAMemoBeforeTheVote(t *testing.T) {
	c, fake := newRouter(t, nil)
	verdict := deliberateAndVote(t, c, testBill())

	if verdict.Vote != models.VoteYes {
		t.Errorf("vote = %q, want Yes", verdict.Vote)
	}
	if got := len(fake.seen("memo")); got != 1 {
		t.Errorf("made %d memo call(s), want 1", got)
	}
	votes := fake.seen("vote")
	if len(votes) != 1 {
		t.Fatalf("made %d vote call(s), want 1", len(votes))
	}
	if got := len(fake.seen("section")); got != 0 {
		t.Errorf("made %d section call(s) for a bill that fits, want 0", got)
	}

	prompt := votes[0].user
	for _, want := range []string{
		"S. 1264",
		"Expedited removal",                    // the statute text itself
		"Sec. 2 funds the program in full.",    // the pro this model wrote
		"Sec. 5 leaves the standard undefined", // the con this model wrote
		"YOUR OWN PROS AND CONS MEMO",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("vote prompt is missing %q", want)
		}
	}
}

// The CRS summary is for the bill cards. No stage of the pipeline may see it.
func TestNoPromptEverCarriesTheCRSSummary(t *testing.T) {
	c, fake := newRouter(t, nil)
	bill := testBill()
	bill.FullText = strings.Repeat(`<section><enum>1.</enum><header>Long</header><text>`+
		strings.Repeat("The Secretary shall promulgate regulations. ", 40)+`</text></section>`, 6)

	// A budget small enough to force the section digest, so the section prompts
	// are checked too.
	c.WithContextBudget(0.75, 4_000)
	deliberateAndVote(t, c, bill)
	if _, err := c.ScoreIdeology(context.Background(), bill); err != nil {
		t.Fatalf("ScoreIdeology: %v", err)
	}

	if len(fake.seen("section")) == 0 {
		t.Fatal("expected the oversized bill to be digested section by section")
	}
	for _, c := range fake.calls {
		for _, banned := range []string{"CRS", "SUMMARY:", bill.Summary} {
			if strings.Contains(c.user, banned) {
				t.Errorf("%s prompt carries %q", c.stage(), banned)
			}
			if strings.Contains(c.system, banned) {
				t.Errorf("%s system prompt carries %q", c.stage(), banned)
			}
		}
	}
}

// Statute text over the model's budget is split, digested by that same model,
// and the memo and vote then run off those notes.
func TestOversizedBillIsDigestedByTheVotingModel(t *testing.T) {
	var logs bytes.Buffer
	c, fake := newRouter(t, nil)
	c.WithLogger(log.New(&logs, "", 0)).WithContextBudget(0.75, 4_000)

	bill := testBill()
	var body strings.Builder
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&body, `<section id="H%d"><enum>%d.</enum><header>Program %d</header><text>%s</text></section>`,
			i, i, i, strings.Repeat("There is authorized to be appropriated such sums as may be necessary. ", 30))
	}
	bill.FullText = body.String()

	m := models.Catalog[0]
	memo, err := c.Deliberate(context.Background(), m, bill)
	if err != nil {
		t.Fatalf("Deliberate: %v", err)
	}
	if !memo.Overflow || memo.Sections < 2 {
		t.Fatalf("memo overflow=%v sections=%d, want the bill digested in pieces", memo.Overflow, memo.Sections)
	}
	if memo.Digest == "" {
		t.Fatal("the model's section notes were not kept for the vote")
	}
	sections := fake.seen("section")
	if len(sections) != memo.Sections {
		t.Errorf("made %d section call(s) for %d section(s)", len(sections), memo.Sections)
	}

	// The memo must be written from the notes, not from the raw text.
	memoCall := fake.seen("memo")[0]
	if !strings.Contains(memoCall.user, "YOUR SECTION NOTES") {
		t.Error("the memo prompt does not use the model's own section notes")
	}
	if strings.Contains(memoCall.user, "authorized to be appropriated such sums") {
		t.Error("the memo prompt still carries the raw text that overflowed")
	}

	if _, err := c.Vote(context.Background(), m, bill, memo); err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if voteCall := fake.seen("vote")[0]; !strings.Contains(voteCall.user, "YOUR SECTION NOTES") {
		t.Error("the vote prompt does not reuse the model's own section notes")
	}
	// The digest is reused rather than recomputed for the vote.
	if got := len(fake.seen("section")); got != memo.Sections {
		t.Errorf("section calls grew to %d during the vote", got)
	}

	out := logs.String()
	for _, want := range []string{"context overflow", bill.ID, m.OpenRouterID, "token budget", "token window"} {
		if !strings.Contains(out, want) {
			t.Errorf("overflow log is missing %q:\n%s", want, out)
		}
	}
}

// Notes are private working papers: each model digests the bill itself, and
// never reads another model's notes.
func TestSectionNotesAreNeverSharedBetweenModels(t *testing.T) {
	c, fake := newRouter(t, func(c call) (string, string) {
		if c.stage() == "section" {
			return fmt.Sprintf(`{"note": "Read by %s."}`, c.model), "stop"
		}
		return defaultReply(c)
	})
	c.WithContextBudget(0.75, 4_000)

	bill := testBill()
	bill.FullText = strings.Repeat(`<section><enum>1.</enum><header>Grants</header><text>`+
		strings.Repeat("Grants shall be awarded competitively. ", 40)+`</text></section>`, 6)

	first, second := models.Catalog[0], models.Catalog[1]
	memoA, err := c.Deliberate(context.Background(), first, bill)
	if err != nil {
		t.Fatalf("Deliberate(%s): %v", first.Key, err)
	}
	memoB, err := c.Deliberate(context.Background(), second, bill)
	if err != nil {
		t.Fatalf("Deliberate(%s): %v", second.Key, err)
	}

	if !strings.Contains(memoA.Digest, first.OpenRouterID) || strings.Contains(memoA.Digest, second.OpenRouterID) {
		t.Errorf("%s digest is not exclusively its own work:\n%s", first.Key, memoA.Digest)
	}
	if !strings.Contains(memoB.Digest, second.OpenRouterID) || strings.Contains(memoB.Digest, first.OpenRouterID) {
		t.Errorf("%s digest is not exclusively its own work:\n%s", second.Key, memoB.Digest)
	}
	for _, sc := range fake.seen("section") {
		if strings.Contains(sc.user, "Read by ") {
			t.Errorf("a section prompt for %s carries another model's note", sc.model)
		}
	}
	if _, err := c.Vote(context.Background(), second, bill, memoA); err != nil {
		// Voting with a foreign memo is a caller mistake the service prevents;
		// the client cannot detect it, so this is documentation, not a check.
		t.Logf("cross-model vote returned %v", err)
	}
}

// The whole bill is sent when it fits: no truncation marker, no 12k ceiling.
func TestStatuteTextIsSentWhole(t *testing.T) {
	c, fake := newRouter(t, nil)
	bill := testBill()
	bill.FullText = `<section><enum>1.</enum><header>Long title</header><text>` +
		strings.Repeat("The Secretary shall establish a pilot program. ", 900) + `</text></section>`

	deliberateAndVote(t, c, bill)

	if got := len(fake.seen("section")); got != 0 {
		t.Fatalf("a %d char bill was split into %d piece(s) despite fitting the window", len(bill.FullText), got)
	}
	memo := fake.seen("memo")[0]
	if !strings.Contains(memo.user, bill.FullText) {
		t.Errorf("the memo prompt does not carry the statute text verbatim (%d chars sent)", len(memo.user))
	}
	if strings.Contains(memo.user, "[text truncated]") {
		t.Error("the statute text was truncated")
	}
}

func TestDeliberateRejectsABillWithoutStatuteText(t *testing.T) {
	c, _ := newRouter(t, nil)
	bill := testBill()
	bill.FullText = ""
	if _, err := c.Deliberate(context.Background(), models.Catalog[0], bill); err == nil {
		t.Fatal("a bill with no statute text must not be deliberated on")
	}
}

func TestVoteRequiresItsOwnMemo(t *testing.T) {
	c, _ := newRouter(t, nil)
	if _, err := c.Vote(context.Background(), models.Catalog[0], testBill(), models.Deliberation{}); err == nil {
		t.Fatal("voting without a memo must fail")
	}
}

func TestVoteParsesFencedJSONAndLooseCasing(t *testing.T) {
	c, _ := newRouter(t, func(c call) (string, string) {
		if c.stage() == "vote" {
			return "Sure!\n```json\n{\"vote\": \"NAY\", \"reason\": \"Too broad.\"}\n```", "stop"
		}
		return defaultReply(c)
	})
	got, err := c.Vote(context.Background(), models.Catalog[0], testBill(), testMemo())
	if err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if got.Vote != models.VoteNo {
		t.Errorf("vote = %q, want No", got.Vote)
	}
}

func TestVoteTrimsRationaleToOneSentence(t *testing.T) {
	c, _ := newRouter(t, func(c call) (string, string) {
		if c.stage() == "vote" {
			return `{"vote":"Yes","reason":"First sentence here. Second sentence should be dropped."}`, "stop"
		}
		return defaultReply(c)
	})
	got, err := c.Vote(context.Background(), models.Catalog[0], testBill(), testMemo())
	if err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if got.Reason != "First sentence here." {
		t.Errorf("reason = %q, want only the first sentence", got.Reason)
	}
}

func TestVoteRejectsUnusableVerdict(t *testing.T) {
	c, _ := newRouter(t, func(c call) (string, string) {
		if c.stage() == "vote" {
			return `{"vote":"maybe","reason":"It depends."}`, "stop"
		}
		return defaultReply(c)
	})
	if _, err := c.Vote(context.Background(), models.Catalog[0], testBill(), testMemo()); err == nil {
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
	c, _ := newRouter(t, func(c call) (string, string) {
		if c.stage() == "vote" {
			return `{"vote": "Yes", "reason": "This.`, "length"
		}
		return defaultReply(c)
	})
	_, err := c.Vote(context.Background(), models.Catalog[0], testBill(), testMemo())
	if err == nil {
		t.Fatal("a rationale cut off after one word should fail the call")
	}
	if !strings.Contains(err.Error(), "ran out of tokens") {
		t.Errorf("err = %v, want it to name the cause", err)
	}
}

// A reply cut short is retried with a larger budget before it is given up on.
func TestTruncatedReplyIsRetriedWithMoreRoom(t *testing.T) {
	var attempts int
	c, fake := newRouter(t, func(c call) (string, string) {
		if c.stage() != "vote" {
			return defaultReply(c)
		}
		attempts++
		if attempts == 1 {
			return `{"vote": "No", "reason": "Prea`, "length"
		}
		return `{"vote":"No","reason":"Preempts state experimentation before the federal approach is proven."}`, "stop"
	})

	got, err := c.Vote(context.Background(), models.Catalog[0], testBill(), testMemo())
	if err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if got.Reason != "Preempts state experimentation before the federal approach is proven." {
		t.Errorf("reason = %q, want the complete second attempt", got.Reason)
	}
	budgets := fake.seen("vote")
	if len(budgets) != 2 {
		t.Fatalf("made %d attempts, want 2", len(budgets))
	}
	if budgets[1].maxTokens <= budgets[0].maxTokens {
		t.Errorf("retry budget %d did not grow from %d", budgets[1].maxTokens, budgets[0].maxTokens)
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

// A memo cut off partway through its lists still has to yield the entries that
// arrived, in the same way a truncated verdict does.
func TestMemoParsingHandlesEveryShape(t *testing.T) {
	cases := map[string]struct {
		reply      string
		pros, cons int
	}{
		"plain":                {`{"pros":["Funds it.","Sunsets it."],"cons":["Preempts states."]}`, 2, 1},
		"fenced":               {"```json\n{\"pros\": [\"Funds it.\"], \"cons\": [\"Costs too much.\"]}\n```", 1, 1},
		"truncated mid entry":  {`{"pros": ["Funds it.", "Sunsets the authority in 2032.", "Adds an aud`, 3, 0},
		"truncated after pros": {`{"pros": ["Funds it."], "cons": [`, 1, 0},
		"chatter around":       {"Here you go:\n{\"pros\":[\"Funds it.\"],\"cons\":[\"Vague.\"]}\nHope that helps.", 1, 1},
		"prose bullets":        {"Pros:\n- Funds the program in full.\n- Sunsets in 2032.\nCons:\n1. Preempts states.\n", 2, 1},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			pros, cons := parseMemo(tc.reply)
			if len(pros) != tc.pros || len(cons) != tc.cons {
				t.Errorf("parseMemo = %v / %v, want %d pro(s) and %d con(s)", pros, cons, tc.pros, tc.cons)
			}
			for _, entry := range append(append([]string{}, pros...), cons...) {
				if strings.HasPrefix(entry, "-") || strings.Contains(entry, `"pros"`) {
					t.Errorf("entry %q still carries scaffolding", entry)
				}
			}
		})
	}
}

func TestDeliberateFailsWhenTheMemoIsEmpty(t *testing.T) {
	c, _ := newRouter(t, func(c call) (string, string) {
		if c.stage() == "memo" {
			return `{"pros": [], "cons": []}`, "stop"
		}
		return defaultReply(c)
	})
	if _, err := c.Deliberate(context.Background(), models.Catalog[0], testBill()); err == nil {
		t.Fatal("an empty memo must fail rather than support a vote")
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
	if _, err := c.Vote(context.Background(), models.Catalog[0], testBill(), testMemo()); err != ErrNoKey {
		t.Errorf("err = %v, want ErrNoKey", err)
	}
	if _, err := c.Deliberate(context.Background(), models.Catalog[0], testBill()); err != ErrNoKey {
		t.Errorf("err = %v, want ErrNoKey", err)
	}
}

func TestScoreIdeologyClampsToRange(t *testing.T) {
	c, _ := newRouter(t, func(c call) (string, string) {
		return `{"score": 4.2}`, "stop"
	})
	score, err := c.ScoreIdeology(context.Background(), testBill())
	if err != nil {
		t.Fatalf("ScoreIdeology: %v", err)
	}
	if score != 1 {
		t.Errorf("score = %v, want it clamped to 1", score)
	}
}

// The budget is a fraction of each model's own window, and the tightest model
// in the catalog must come out tightest here too.
func TestTextBudgetTracksEachModelsWindow(t *testing.T) {
	c := New("http://example.invalid", "k", "", time.Second)
	for _, m := range models.Catalog {
		budget := c.textBudgetChars(m)
		want := int(float64(m.ContextTokens)*defaultBudgetRatio) - reserveTokens
		if budget != want*4 {
			t.Errorf("%s budget = %d chars, want %d", m.Key, budget, want*4)
		}
		if budget >= m.ContextTokens*4 {
			t.Errorf("%s budget %d leaves no headroom in a %d token window", m.Key, budget, m.ContextTokens)
		}
	}

	grok, _ := models.ModelByKey("grok")
	opus, _ := models.ModelByKey("opus")
	if c.textBudgetChars(grok) >= c.textBudgetChars(opus) {
		t.Error("Grok has the tightest window in the catalog and should have the tightest budget")
	}
}
