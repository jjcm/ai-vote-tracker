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
	"strings"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// ErrNoKey is returned when the client is used without an API key.
var ErrNoKey = errors.New("openrouter: OPENROUTER_KEY is not set")

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

	verdict, err := c.complete(ctx, model.OpenRouterID, voteSystemPrompt, b.String(), 300)
	if err != nil {
		return Verdict{}, err
	}

	vote := models.NormalizeVote(verdict.Vote)
	if vote == "" {
		return Verdict{}, fmt.Errorf("openrouter: %s returned an unusable vote %q", model.Name, verdict.Vote)
	}
	reason := firstSentence(verdict.Reason)
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

	raw, err := c.completeRaw(ctx, scorer.OpenRouterID, ideologySystemPrompt, b.String(), 200)
	if err != nil {
		return 0, err
	}
	var parsed struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal([]byte(extractJSON(raw)), &parsed); err != nil {
		return 0, fmt.Errorf("openrouter: parse ideology score: %w", err)
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

func (c *Client) complete(ctx context.Context, model, system, user string, maxTokens int) (rawVerdict, error) {
	content, err := c.completeRaw(ctx, model, system, user, maxTokens)
	if err != nil {
		return rawVerdict{}, err
	}
	var v rawVerdict
	if err := json.Unmarshal([]byte(extractJSON(content)), &v); err != nil {
		// Some models ignore the JSON instruction and answer in prose; salvage
		// the verdict rather than throwing the call away.
		if vote := sniffVote(content); vote != "" {
			return rawVerdict{Vote: vote, Reason: firstSentence(content)}, nil
		}
		return rawVerdict{}, fmt.Errorf("openrouter: parse response from %s: %w", model, err)
	}
	return v, nil
}

func (c *Client) completeRaw(ctx context.Context, model, system, user string, maxTokens int) (string, error) {
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
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt*attempt) * 1500 * time.Millisecond
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(delay):
			}
		}
		content, retryable, err := c.do(ctx, payload)
		if err == nil {
			return content, nil
		}
		lastErr = err
		if !retryable {
			return "", err
		}
	}
	return "", lastErr
}

func (c *Client) do(ctx context.Context, payload chatRequest) (content string, retryable bool, err error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", false, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if c.referer != "" {
		req.Header.Set("HTTP-Referer", c.referer)
	}
	req.Header.Set("X-Title", "AI Vote Tracker")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", true, fmt.Errorf("openrouter: request %s: %w", payload.Model, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", true, fmt.Errorf("openrouter: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return "", retry, fmt.Errorf("openrouter: %s returned HTTP %d: %s", payload.Model, resp.StatusCode, snippet(raw))
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", true, fmt.Errorf("openrouter: decode response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return "", false, fmt.Errorf("openrouter: %s: %s", payload.Model, parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", true, fmt.Errorf("openrouter: %s returned no choices", payload.Model)
	}
	content = strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", true, fmt.Errorf("openrouter: %s returned empty content", payload.Model)
	}
	return content, false, nil
}

// extractJSON pulls the first JSON object out of a model reply, tolerating
// markdown fences and surrounding chatter.
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if fence := strings.Index(s, "```"); fence >= 0 {
		rest := s[fence+3:]
		if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			rest = rest[:end]
		}
		s = strings.TrimSpace(rest)
	}
	start := strings.IndexByte(s, '{')
	end := strings.LastIndexByte(s, '}')
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}

func sniffVote(s string) string {
	lower := strings.ToLower(s)
	yes := strings.Index(lower, "yes")
	no := strings.Index(lower, "no")
	switch {
	case yes >= 0 && (no < 0 || yes < no):
		return models.VoteYes
	case no >= 0:
		return models.VoteNo
	}
	return ""
}

// firstSentence trims a rationale down to a single sentence.
func firstSentence(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	s = strings.Trim(s, "\"' ")
	for i, r := range s {
		if r == '.' || r == '!' || r == '?' {
			// Keep decimals and abbreviations like "U.S." intact.
			if i+1 < len(s) && s[i+1] != ' ' {
				continue
			}
			return strings.TrimSpace(s[:i+1])
		}
	}
	if s != "" && !strings.HasSuffix(s, ".") {
		s += "."
	}
	return s
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
