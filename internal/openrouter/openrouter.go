// Package openrouter talks to the OpenRouter chat completions API to collect a
// binary Yes/No verdict and a one-sentence rationale from each model.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// ErrNoKey is returned when the client is used without an API key.
var ErrNoKey = errors.New("openrouter: OPENROUTER_KEY is not set")

const (
	maxAttempts = 3
	// Reasoning models spend part of the completion budget on hidden thinking,
	// so the ceiling has to leave room for the answer behind it.
	voteMaxTokens     = 700
	ideologyMaxTokens = 400
	finishLength      = "length"
)

// Client is a minimal OpenRouter chat completions client.
type Client struct {
	baseURL string
	apiKey  string
	referer string
	http    *http.Client
}

// New builds a client. baseURL should include the /api/v1 suffix.
func New(baseURL, apiKey, referer string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		referer: referer,
		http:    &http.Client{Timeout: timeout},
	}
}

// Enabled reports whether an API key is configured.
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

// Verdict is a model's parsed answer.
type Verdict struct {
	Vote   string
	Reason string
}

const voteSystemPrompt = `You are a legislative analyst casting a recorded floor vote on a bill before the United States Congress.

Read the bill and decide how you would vote on final passage. You must pick a side: there is no present, abstain, or undecided option.

Respond with a single JSON object and nothing else:
{"vote": "Yes" | "No", "reason": "<one sentence>"}

Rules for "reason":
- Exactly one sentence, at most 25 words.
- State the decisive consideration in your own voice, e.g. "Strengthens border security while preserving due process for valid asylum claims."
- No preamble, no markdown, no quotes around the sentence, no restating the bill title.`

// Vote asks one model how it would vote on a bill.
func (c *Client) Vote(ctx context.Context, model models.Model, bill models.Bill) (Verdict, error) {
	if !c.Enabled() {
		return Verdict{}, ErrNoKey
	}

	var b strings.Builder
	fmt.Fprintf(&b, "BILL: %s — %s\n", bill.Number, bill.Title)
	fmt.Fprintf(&b, "CHAMBER: %s\n", bill.Chamber)
	if bill.PolicyArea != "" {
		fmt.Fprintf(&b, "POLICY AREA: %s\n", bill.PolicyArea)
	}
	if bill.Status != "" {
		fmt.Fprintf(&b, "LATEST ACTION: %s\n", bill.Status)
	}
	if bill.Summary != "" {
		fmt.Fprintf(&b, "\nSUMMARY:\n%s\n", bill.Summary)
	}
	if text := truncate(bill.FullText, 12000); text != "" {
		fmt.Fprintf(&b, "\nBILL TEXT:\n%s\n", text)
	}
	b.WriteString("\nHow would you vote on final passage of this bill?")

	verdict, truncated, err := c.complete(ctx, model.OpenRouterID, voteSystemPrompt, b.String(), voteMaxTokens)
	if err != nil {
		return Verdict{}, err
	}

	vote := models.NormalizeVote(verdict.Vote)
	if vote == "" {
		return Verdict{}, fmt.Errorf("openrouter: %s returned an unusable vote %q", model.Name, verdict.Vote)
	}
	// A reply that was still being written when the token budget ran out can
	// leave a stub of a sentence behind. Fail the call so the next round
	// retries it rather than publishing half a thought.
	if truncated && !usableReason(verdict.Reason) {
		return Verdict{}, fmt.Errorf("openrouter: %s ran out of tokens before finishing its rationale", model.Name)
	}
	reason := verdict.Reason
	if reason == "" {
		reason = "No rationale returned."
	}
	return Verdict{Vote: vote, Reason: reason}, nil
}

const ideologySystemPrompt = `You are a nonpartisan political scientist scoring the ideological direction of United States legislation.

Score the bill on a scale from -1.0 to +1.0:
-1.0 = strongly progressive/liberal, 0.0 = genuinely centrist or bipartisan, +1.0 = strongly conservative.

Judge the policy content, not the sponsor's party.

Respond with a single JSON object and nothing else:
{"score": <number between -1 and 1>, "reason": "<one short sentence>"}`

// ScoreIdeology places a bill on the -1..+1 ideology axis used by the alignment
// page. It uses the last model in the catalog, which is the cheapest.
func (c *Client) ScoreIdeology(ctx context.Context, bill models.Bill) (float64, error) {
	if !c.Enabled() {
		return 0, ErrNoKey
	}
	scorer := models.Catalog[len(models.Catalog)-1]

	var b strings.Builder
	fmt.Fprintf(&b, "BILL: %s — %s\n", bill.Number, bill.Title)
	if bill.PolicyArea != "" {
		fmt.Fprintf(&b, "POLICY AREA: %s\n", bill.PolicyArea)
	}
	if bill.Summary != "" {
		fmt.Fprintf(&b, "\nSUMMARY:\n%s\n", bill.Summary)
	}
	if text := truncate(bill.FullText, 6000); text != "" {
		fmt.Fprintf(&b, "\nBILL TEXT:\n%s\n", text)
	}

	raw, _, err := c.completeRaw(ctx, scorer.OpenRouterID, ideologySystemPrompt, b.String(), ideologyMaxTokens)
	if err != nil {
		return 0, err
	}
	body, _, ok := jsonObject(raw)
	if !ok {
		return 0, fmt.Errorf("openrouter: no JSON object in the ideology reply: %s", snippet([]byte(raw)))
	}
	var parsed struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		// The object may have been cut off after the score was written.
		value, ok := jsonNumberField(body, "score")
		if !ok {
			return 0, fmt.Errorf("openrouter: parse ideology score: %w", err)
		}
		parsed.Score = value
	}
	return clamp(parsed.Score, -1, 1), nil
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	MaxTokens      int             `json:"max_tokens"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    any    `json:"code"`
	} `json:"error"`
}

type rawVerdict struct {
	Vote   string `json:"vote"`
	Reason string `json:"reason"`
}

func (c *Client) complete(ctx context.Context, model, system, user string, maxTokens int) (rawVerdict, bool, error) {
	content, truncated, err := c.completeRaw(ctx, model, system, user, maxTokens)
	if err != nil {
		return rawVerdict{}, false, err
	}
	v, err := parseVerdict(content)
	if err != nil {
		return rawVerdict{}, truncated, fmt.Errorf("openrouter: parse response from %s: %w", model, err)
	}
	return v, truncated, nil
}

// parseVerdict turns a model reply into a verdict. Models wrap the JSON in
// markdown fences, prefix it with chatter, answer in prose instead, or stop
// mid-string when the token budget runs out — all of which have to yield a
// clean rationale or none at all, never the response envelope itself.
func parseVerdict(content string) (rawVerdict, error) {
	if body, _, ok := jsonObject(content); ok {
		var v rawVerdict
		if err := json.Unmarshal([]byte(body), &v); err == nil {
			return rawVerdict{Vote: v.Vote, Reason: cleanReason(v.Reason)}, nil
		}
		// Malformed or cut off before the closing brace: read the fields that
		// did make it out.
		if vote, ok := jsonStringField(body, "vote"); ok {
			reason, _ := jsonStringField(body, "reason")
			return rawVerdict{Vote: vote, Reason: cleanReason(reason)}, nil
		}
	}
	// No usable JSON: some models simply answer in prose.
	if vote := sniffVote(content); vote != "" {
		return rawVerdict{Vote: vote, Reason: cleanReason(content)}, nil
	}
	return rawVerdict{}, fmt.Errorf("no verdict in reply: %s", snippet([]byte(content)))
}

// completeRaw returns the assistant message, and whether the provider stopped
// it early because it hit the token ceiling.
func (c *Client) completeRaw(ctx context.Context, model, system, user string, maxTokens int) (string, bool, error) {
	payload := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
		Temperature:    0.2,
		MaxTokens:      maxTokens,
		ResponseFormat: &responseFormat{Type: "json_object"},
	}

	var lastErr error
	var salvaged string
	for attempt := 0; attempt < maxAttempts; attempt++ {
		payload.MaxTokens = maxTokens
		content, finish, retryable, err := c.do(ctx, payload)
		if err != nil {
			lastErr = err
			if !retryable {
				return "", false, err
			}
			if attempt < maxAttempts-1 {
				delay := time.Duration(attempt*attempt) * 1500 * time.Millisecond
				select {
				case <-ctx.Done():
					return "", false, ctx.Err()
				case <-time.After(delay):
				}
			}
			continue
		}
		if finish != finishLength {
			return content, false, nil
		}
		// The reply was cut off. On reasoning models the hidden thinking eats
		// the budget before the answer is written, so retry with more room and
		// keep this attempt in case the next one fails outright.
		salvaged, lastErr = content, nil
		maxTokens *= 2
	}

	if salvaged != "" {
		return salvaged, true, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("openrouter: %s produced no usable reply", model)
	}
	return "", false, lastErr
}

func (c *Client) do(ctx context.Context, payload chatRequest) (content, finishReason string, retryable bool, err error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", "", false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if c.referer != "" {
		req.Header.Set("HTTP-Referer", c.referer)
	}
	req.Header.Set("X-Title", "AI Vote Tracker")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", true, fmt.Errorf("openrouter: request %s: %w", payload.Model, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", "", true, fmt.Errorf("openrouter: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", "", retry, fmt.Errorf("openrouter: %s returned HTTP %d: %s", payload.Model, resp.StatusCode, snippet(raw))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", "", true, fmt.Errorf("openrouter: decode response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", "", false, fmt.Errorf("openrouter: %s: %s", payload.Model, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", "", true, fmt.Errorf("openrouter: %s returned no choices", payload.Model)
	}
	choice := parsed.Choices[0]
	content = strings.TrimSpace(choice.Message.Content)
	if content == "" {
		return "", choice.FinishReason, true, fmt.Errorf("openrouter: %s returned empty content", payload.Model)
	}
	return content, choice.FinishReason, false, nil
}

// jsonObject isolates the first JSON object in a model reply, tolerating
// markdown fences and chatter on either side. An object with no closing brace
// is returned anyway, flagged as truncated, so the fields written before the
// cut can still be recovered.
func jsonObject(s string) (body string, truncated bool, ok bool) {
	s = stripFences(s)
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false, false
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		switch ch := s[i]; {
		case escaped:
			escaped = false
		case inString && ch == '\\':
			escaped = true
		case ch == '"':
			inString = !inString
		case inString:
			// Braces inside a string value are not structure.
		case ch == '{':
			depth++
		case ch == '}':
			depth--
			if depth == 0 {
				return s[start : i+1], false, true
			}
		}
	}
	return s[start:], true, true
}

func stripFences(s string) string {
	s = strings.TrimSpace(s)
	fence := strings.Index(s, "```")
	if fence < 0 {
		return s
	}
	rest := s[fence+3:]
	// Drop the language tag that usually follows the opening fence.
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		if tag := strings.TrimSpace(rest[:nl]); !strings.ContainsAny(tag, "{\"") {
			rest = rest[nl+1:]
		}
	}
	if end := strings.Index(rest, "```"); end >= 0 {
		rest = rest[:end]
	}
	return strings.TrimSpace(rest)
}

// jsonStringField reads one string field out of a JSON object body without
// requiring the object to be well formed, so a value that was cut off before
// its closing quote still comes back whole up to the cut.
func jsonStringField(body, key string) (string, bool) {
	rest, ok := seekValue(body, key)
	if !ok || !strings.HasPrefix(rest, `"`) {
		return "", false
	}

	end := -1
	for i := 1; i < len(rest); i++ {
		if rest[i] == '\\' {
			i++
			continue
		}
		if rest[i] == '"' {
			end = i
			break
		}
	}

	quoted := rest
	if end >= 0 {
		quoted = rest[:end+1]
	} else {
		// Truncated mid-string. Close the quote ourselves, dropping a dangling
		// backslash first so it cannot escape the quote we are adding.
		quoted = strings.TrimRight(quoted, "\\") + `"`
	}
	if out, err := strconv.Unquote(quoted); err == nil {
		return out, true
	}
	return strings.Trim(quoted, `"`), true
}

// jsonNumberField is the numeric counterpart of jsonStringField.
func jsonNumberField(body, key string) (float64, bool) {
	rest, ok := seekValue(body, key)
	if !ok {
		return 0, false
	}
	end := strings.IndexFunc(rest, func(r rune) bool {
		return !strings.ContainsRune("+-.eE0123456789", r)
	})
	if end < 0 {
		end = len(rest)
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(rest[:end]), 64)
	if err != nil {
		return 0, false
	}
	return value, true
}

// seekValue positions just after the colon following "key".
func seekValue(body, key string) (string, bool) {
	i := strings.Index(body, `"`+key+`"`)
	if i < 0 {
		return "", false
	}
	rest := strings.TrimLeft(body[i+len(key)+2:], " \t\r\n")
	if !strings.HasPrefix(rest, ":") {
		return "", false
	}
	return strings.TrimLeft(rest[1:], " \t\r\n"), true
}

// cleanReason reduces a rationale to a single readable sentence. It is the last
// line of defence against response scaffolding reaching the page: anything that
// still looks like the JSON envelope is unwrapped, and dropped if it cannot be.
func cleanReason(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "{") || strings.Contains(s, `"reason"`) {
		inner, ok := jsonStringField(s, "reason")
		if !ok {
			return ""
		}
		s = strings.TrimSpace(inner)
	}
	s = strings.TrimSpace(strings.Trim(s, "`"))
	if s == "" || strings.HasPrefix(s, "{") || strings.HasPrefix(s, `"vote"`) {
		return ""
	}
	return firstSentence(s)
}

// usableReason rejects a rationale that is obviously a fragment.
func usableReason(s string) bool {
	return len(strings.Fields(s)) >= 4
}

var voteWordRE = regexp.MustCompile(`(?i)\b(yes|no|yea|nay|aye|support|oppose)\b`)

// sniffVote recovers a verdict from a prose reply. It matches whole words so
// that "nonetheless" or "notably" cannot be read as a No.
func sniffVote(s string) string {
	return models.NormalizeVote(voteWordRE.FindString(s))
}

// abbreviations are the trailing tokens whose full stop does not end a
// sentence. Single letters ("U.S.", "J. Smith") are handled separately.
var abbreviations = map[string]bool{
	"u.s": true, "u.k": true, "e.g": true, "i.e": true, "etc": true, "vs": true,
	"mr": true, "mrs": true, "ms": true, "dr": true, "sen": true, "rep": true,
	"sec": true, "no": true, "fig": true, "approx": true, "st": true, "jr": true,
}

// firstSentence trims a rationale down to a single sentence.
func firstSentence(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.Trim(s, "\"' ")
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.', '!', '?':
		default:
			continue
		}
		// A stop that runs straight into more text is a decimal or an
		// initialism, not a sentence boundary.
		if i+1 < len(s) && s[i+1] != ' ' {
			continue
		}
		if s[i] == '.' && endsWithAbbreviation(s[:i]) {
			continue
		}
		return strings.TrimSpace(s[:i+1])
	}
	if s != "" && !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
}

func endsWithAbbreviation(head string) bool {
	token := head
	if cut := strings.LastIndexAny(head, " ("); cut >= 0 {
		token = head[cut+1:]
	}
	token = strings.ToLower(strings.Trim(token, `"'`))
	return len(token) == 1 || abbreviations[token]
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "\n[text truncated]"
}

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
