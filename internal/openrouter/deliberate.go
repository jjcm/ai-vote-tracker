package openrouter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/billtext"
	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// Every model runs the same pipeline over the statute text, and runs all of it
// itself:
//
//  1. if the bill text does not fit its context window, it summarizes the bill
//     section by section into notes only it will ever see;
//  2. it writes a pros and cons memo from the text (or from its own notes);
//  3. it votes on final passage from that memo and that text.
//
// No stage is ever fed a CRS summary, and no model is ever shown another
// model's notes or memo.

const (
	sectionMaxTokens = 1200
	memoMaxTokens    = 1800
	voteMaxTokens    = 700

	// reserveTokens is the room kept back from the context window for the
	// system prompt, the bill header, the memo, and the completion.
	reserveTokens = 8000
	// minTextTokens keeps a small or misconfigured window from producing a
	// budget too tight to send anything at all.
	minTextTokens = 1500

	defaultBudgetRatio   = 0.75
	defaultContextTokens = 128_000

	// ideologyTextChars caps the statute text sent for the alignment score,
	// which is a cheap one-shot classification rather than a deliberation.
	ideologyTextChars = 60_000
)

// ErrNoStatuteText is returned for a bill whose statute text never arrived.
// Such a bill is not put to a vote: the models read the law or nothing.
var ErrNoStatuteText = errors.New("openrouter: the bill carries no statute text")

// ErrStaleMemo reports that a cached memo cannot be voted from: the bill text
// no longer fits the model's budget and the memo has no section notes to fall
// back on. The caller re-runs the deliberation.
var ErrStaleMemo = errors.New("openrouter: the cached memo has no section notes for an oversized bill")

const sectionSystemPrompt = `You are reading the text of a bill before the United States Congress and writing a SECTION NOTE for your own later use. You are reading the law itself — raw legislative text, often XML — not news coverage and not anyone's summary of it.

You will be shown one part of the bill. Record what that text actually does: the duties it creates, whom it binds, the money it authorizes, the deadlines, thresholds and standards it sets, and anything in the drafting a legislator should notice before voting.

Respond with a single JSON object and nothing else:
{"note": "<up to 150 words>"}

Rules for "note":
- Report only what this part of the text says. Do not speculate about the parts you have not been shown.
- Keep section numbers, dollar figures, and dates: they are what makes the note worth having later.
- No preamble, no markdown.`

const memoSystemPrompt = `You are legislative counsel reading the text of a bill before the United States Congress. Before you vote, you write your own pros and cons memo — nobody else's.

Weigh the operative text in front of you: the obligations it imposes, the authorities it grants, the money it moves, the deadlines it sets, and the drafting problems it leaves behind. You are working from the law itself. You have not been given a committee report, a press account, or a third-party summary, and you must not invent one.

Respond with a single JSON object and nothing else:
{"pros": ["<one sentence>", "..."], "cons": ["<one sentence>", "..."]}

Rules:
- Three to six entries in each list, and neither list may be empty: if the bill looks overwhelmingly good or bad to you, still name the strongest argument on the other side.
- One sentence per entry, at most 30 words, in your own voice.
- Point at the section a claim comes from where the text lets you, e.g. "Sec. 5 raises the credible fear standard to more likely than not."
- No preamble, no markdown, no numbering.`

const voteSystemPrompt = `You are a legislative analyst casting a recorded floor vote on a bill before the United States Congress.

You have the text of the law — the bill's own text, or the section notes you took while reading it — together with the pros and cons memo you wrote yourself. Decide how you would vote on final passage. You must pick a side: there is no present, abstain, or undecided option.

Respond with a single JSON object and nothing else:
{"vote": "Yes" | "No", "reason": "<one sentence>"}

Rules for "reason":
- Exactly one sentence, at most 25 words.
- State the decisive consideration in your own voice, e.g. "Strengthens border security while preserving due process for valid asylum claims."
- No preamble, no markdown, no quotes around the sentence, no restating the bill title.`

const (
	statuteHeading = "STATUTE TEXT (the bill itself, verbatim)"
	digestHeading  = "YOUR SECTION NOTES (written by you while reading this bill's text)"
)

// Verdict is a model's parsed answer on final passage.
type Verdict struct {
	Vote   string
	Reason string
}

// Deliberate has one model read the bill and write its own pros and cons memo.
// When the statute text overruns that model's context budget, the model first
// digests the bill section by section and the memo is written from its own
// notes, which travel back in the returned Deliberation so the vote can reuse
// them without paying for the digest twice.
func (c *Client) Deliberate(ctx context.Context, m models.Model, bill models.Bill) (models.Deliberation, error) {
	if !c.Enabled() {
		return models.Deliberation{}, ErrNoKey
	}
	ev, err := c.gather(ctx, m, bill)
	if err != nil {
		return models.Deliberation{}, err
	}
	pros, cons, err := c.prosAndCons(ctx, m, bill, ev)
	if err != nil {
		return models.Deliberation{}, err
	}
	return models.Deliberation{
		BillID:    bill.ID,
		ModelKey:  m.Key,
		ModelName: m.Name,
		Pros:      pros,
		Cons:      cons,
		Digest:    ev.digest,
		Sections:  ev.sections,
		Overflow:  ev.overflow,
		TextHash:  bill.TextFingerprint(),
		CreatedAt: time.Now().UTC(),
	}, nil
}

// Vote asks one model how it would vote on final passage, given the statute
// text (or its own section notes) and the memo it wrote in Deliberate.
func (c *Client) Vote(ctx context.Context, m models.Model, bill models.Bill, memo models.Deliberation) (Verdict, error) {
	if !c.Enabled() {
		return Verdict{}, ErrNoKey
	}
	if !memo.Usable() {
		return Verdict{}, fmt.Errorf("openrouter: %s has no pros and cons memo to vote on %s from", m.Name, bill.ID)
	}
	ev, err := c.recall(m, bill, memo)
	if err != nil {
		return Verdict{}, err
	}

	var b strings.Builder
	writeBillHeader(&b, bill)
	fmt.Fprintf(&b, "\n%s:\n%s\n", ev.heading, ev.body)
	fmt.Fprintf(&b, "\nYOUR OWN PROS AND CONS MEMO ON THIS BILL:\n%s\n", renderMemo(memo))
	b.WriteString("\nHow would you vote on final passage of this bill?")

	verdict, truncated, err := c.complete(ctx, m.OpenRouterID, voteSystemPrompt, b.String(), voteMaxTokens)
	if err != nil {
		return Verdict{}, err
	}

	vote := models.NormalizeVote(verdict.Vote)
	if vote == "" {
		return Verdict{}, fmt.Errorf("openrouter: %s returned an unusable vote %q", m.Name, verdict.Vote)
	}
	// A reply that was still being written when the token budget ran out can
	// leave a stub of a sentence behind. Fail the call so the next round
	// retries it rather than publishing half a thought.
	if truncated && !usableReason(verdict.Reason) {
		return Verdict{}, fmt.Errorf("openrouter: %s ran out of tokens before finishing its rationale", m.Name)
	}
	reason := verdict.Reason
	if reason == "" {
		reason = "No rationale returned."
	}
	return Verdict{Vote: vote, Reason: reason}, nil
}

// evidence is the statute text one model reasons from: either the bill text
// itself, or — when that will not fit the model's context window — the section
// notes that same model took on it.
type evidence struct {
	heading  string
	body     string
	digest   string
	sections int
	overflow bool
}

// gather assembles the evidence for one model, digesting the bill section by
// section if it has to.
func (c *Client) gather(ctx context.Context, m models.Model, bill models.Bill) (evidence, error) {
	text := strings.TrimSpace(bill.FullText)
	if text == "" {
		return evidence{}, fmt.Errorf("%w: %s", ErrNoStatuteText, bill.ID)
	}
	budget := c.textBudgetChars(m)
	if len(text) <= budget {
		return evidence{heading: statuteHeading, body: text}, nil
	}

	c.logf("context overflow: %s statute text is %d chars (~%d tokens), over the %d token budget for %s (%d token window) — splitting into sections",
		bill.ID, len(text), billtext.EstimateTokens(text), budget/billtext.CharsPerToken,
		m.OpenRouterID, c.contextWindow(m))

	// Half the budget per piece leaves the model room for the prompt around
	// each section and for the note it writes back.
	sections := billtext.Split(text, budget/2)
	c.logf("context overflow: %s split into %d section(s) for %s to digest itself", bill.ID, len(sections), m.OpenRouterID)

	notes := make([]string, 0, len(sections))
	for i, s := range sections {
		note, err := c.sectionNote(ctx, m, bill, s, i+1, len(sections))
		if err != nil {
			return evidence{}, err
		}
		notes = append(notes, fmt.Sprintf("%d. %s\n%s", i+1, s.Label, note))
	}

	digest := strings.Join(notes, "\n\n")
	c.logf("context overflow: %s digested by %s into %d section note(s), %d chars (~%d tokens)",
		bill.ID, m.OpenRouterID, len(sections), len(digest), billtext.EstimateTokens(digest))

	// Notes that somehow overrun the budget themselves are clipped rather than
	// sent to be rejected.
	if len(digest) > budget {
		c.logf("context overflow: %s section notes still exceed the budget for %s; clipping to %d chars",
			bill.ID, m.OpenRouterID, budget)
		digest = clip(digest, budget)
	}
	return evidence{
		heading:  digestHeading,
		body:     digest,
		digest:   digest,
		sections: len(sections),
		overflow: true,
	}, nil
}

// recall rebuilds the evidence for the vote from work already done: the
// model's own section notes when the bill needed them, otherwise the bill text
// itself. It never calls the provider.
func (c *Client) recall(m models.Model, bill models.Bill, memo models.Deliberation) (evidence, error) {
	if memo.Digest != "" {
		return evidence{
			heading:  digestHeading,
			body:     memo.Digest,
			digest:   memo.Digest,
			sections: memo.Sections,
			overflow: true,
		}, nil
	}
	text := strings.TrimSpace(bill.FullText)
	if text == "" {
		return evidence{}, fmt.Errorf("%w: %s", ErrNoStatuteText, bill.ID)
	}
	// The memo travels in the same request, so it comes out of the same budget.
	if len(text)+len(renderMemo(memo)) > c.textBudgetChars(m) {
		return evidence{}, ErrStaleMemo
	}
	return evidence{heading: statuteHeading, body: text}, nil
}

// sectionNote has the model summarize one piece of the bill for its own use.
func (c *Client) sectionNote(ctx context.Context, m models.Model, bill models.Bill, s billtext.Section, index, total int) (string, error) {
	var b strings.Builder
	writeBillHeader(&b, bill)
	fmt.Fprintf(&b, "\nYou are reading part %d of %d of this bill's text.\n", index, total)
	fmt.Fprintf(&b, "PART %d — %s\n\n%s\n", index, s.Label, s.Text)
	b.WriteString("\nWrite your section note on this part of the text.")

	raw, _, err := c.completeRaw(ctx, m.OpenRouterID, sectionSystemPrompt, b.String(), sectionMaxTokens)
	if err != nil {
		return "", fmt.Errorf("openrouter: %s could not digest %q of %s: %w", m.Name, s.Label, bill.ID, err)
	}
	note := firstString(raw, "note")
	if note == "" {
		// A note written as prose is still a note.
		note = strings.TrimSpace(stripFences(raw))
	}
	if note == "" {
		return "", fmt.Errorf("openrouter: %s returned an empty note for %q of %s", m.Name, s.Label, bill.ID)
	}
	return note, nil
}

// prosAndCons has the model argue both sides of the bill to itself.
func (c *Client) prosAndCons(ctx context.Context, m models.Model, bill models.Bill, ev evidence) (pros, cons []string, err error) {
	var b strings.Builder
	writeBillHeader(&b, bill)
	fmt.Fprintf(&b, "\n%s:\n%s\n", ev.heading, ev.body)
	b.WriteString("\nWrite your pros and cons memo on this bill.")

	raw, _, err := c.completeRaw(ctx, m.OpenRouterID, memoSystemPrompt, b.String(), memoMaxTokens)
	if err != nil {
		return nil, nil, err
	}
	pros, cons = parseMemo(raw)
	if len(pros) == 0 && len(cons) == 0 {
		return nil, nil, fmt.Errorf("openrouter: %s returned no pros or cons for %s: %s", m.Name, bill.ID, snippet([]byte(raw)))
	}
	return pros, cons, nil
}

// parseMemo pulls the two lists out of a memo reply, falling back to the
// "Pros: … Cons: …" prose shape when there is no JSON to read.
func parseMemo(content string) (pros, cons []string) {
	body, _, ok := jsonObject(content)
	if !ok {
		body = content
	}
	pros, _ = jsonStringArrayField(body, "pros")
	cons, _ = jsonStringArrayField(body, "cons")
	if len(pros) == 0 && len(cons) == 0 {
		pros, cons = proseMemo(content)
	}
	return tidyList(pros), tidyList(cons)
}

// proseMemo reads a memo a model wrote as headed bullet lists.
func proseMemo(content string) (pros, cons []string) {
	var current *[]string
	for _, line := range strings.Split(stripFences(content), "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		switch {
		case strings.HasPrefix(lower, "pros"):
			current = &pros
			continue
		case strings.HasPrefix(lower, "cons"):
			current = &cons
			continue
		}
		if current == nil || line == "" {
			continue
		}
		entry := strings.TrimLeft(line, "-*•0123456789. \t")
		if entry != "" {
			*current = append(*current, entry)
		}
	}
	return pros, cons
}

// tidyList normalises memo entries and caps how many of them are kept.
func tidyList(in []string) []string {
	const maxEntries = 8
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(s), "-*• \t"))
		if s == "" {
			continue
		}
		out = append(out, clip(s, 300))
		if len(out) == maxEntries {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// renderMemo lays the memo out for the model that wrote it.
func renderMemo(memo models.Deliberation) string {
	var b strings.Builder
	b.WriteString("PROS:\n")
	if len(memo.Pros) == 0 {
		b.WriteString("- (you recorded none)\n")
	}
	for _, p := range memo.Pros {
		fmt.Fprintf(&b, "- %s\n", p)
	}
	b.WriteString("CONS:\n")
	if len(memo.Cons) == 0 {
		b.WriteString("- (you recorded none)\n")
	}
	for _, c := range memo.Cons {
		fmt.Fprintf(&b, "- %s\n", c)
	}
	return strings.TrimRight(b.String(), "\n")
}

// writeBillHeader states which bill is on the floor. It deliberately carries
// no summary: the models are reading the law, and a CRS summary would be
// somebody else's reading of it.
func writeBillHeader(b *strings.Builder, bill models.Bill) {
	fmt.Fprintf(b, "BILL: %s — %s\n", bill.Number, bill.Title)
	fmt.Fprintf(b, "CHAMBER: %s\n", bill.Chamber)
	if bill.PolicyArea != "" {
		fmt.Fprintf(b, "POLICY AREA: %s\n", bill.PolicyArea)
	}
	if bill.Status != "" {
		fmt.Fprintf(b, "LATEST ACTION: %s\n", bill.Status)
	}
	if bill.TextVersion != "" {
		fmt.Fprintf(b, "TEXT VERSION: %s\n", bill.TextVersion)
	}
}

// contextWindow is the window the client will assume for a model.
func (c *Client) contextWindow(m models.Model) int {
	if c.contextTokens > 0 {
		return c.contextTokens
	}
	if m.ContextTokens > 0 {
		return m.ContextTokens
	}
	return defaultContextTokens
}

// textBudgetChars is how much statute text may go into a single request to a
// model: a fraction of its context window, less the room the prompts and the
// completion need.
func (c *Client) textBudgetChars(m models.Model) int {
	ratio := c.budgetRatio
	if ratio <= 0 || ratio > 1 {
		ratio = defaultBudgetRatio
	}
	tokens := int(float64(c.contextWindow(m))*ratio) - reserveTokens
	if tokens < minTextTokens {
		tokens = minTextTokens
	}
	return tokens * billtext.CharsPerToken
}
