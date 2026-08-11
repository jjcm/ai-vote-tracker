// Package openrouter talks to the OpenRouter chat completions API. Each model
// reads the statute text of a bill, writes its own pros and cons memo, and
// then casts a binary Yes/No vote on final passage from that memo.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// ErrNoKey is returned when the client is used without an API key.
var ErrNoKey = errors.New("openrouter: OPENROUTER_KEY is not set")

const (
	// maxAttempts is deliberately patient. Nothing is waiting on a verdict, and
	// giving up early on a rate limit or a reply cut off mid-rationale costs a
	// whole deliberation, which is far more expensive than waiting.
	maxAttempts       = 4
	ideologyMaxTokens = 400
	finishLength      = "length"
)

// Client is a minimal OpenRouter chat completions client.
type Client struct {
	baseURL string
	apiKey  string
	referer string
	http    *http.Client

	budgetRatio   float64
	contextTokens int
	logger        *log.Logger
}

// New builds a client. baseURL should include the /api/v1 suffix.
func New(baseURL, apiKey, referer string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &Client{
		baseURL:     strings.TrimRight(baseURL, "/"),
		apiKey:      apiKey,
		referer:     referer,
		http:        &http.Client{Timeout: timeout},
		budgetRatio: defaultBudgetRatio,
	}
}

// WithLogger routes the client's own diagnostics — chiefly context overflow —
// to a logger.
func (c *Client) WithLogger(logger *log.Logger) *Client {
	c.logger = logger
	return c
}

// WithContextBudget sets the fraction of each model's context window that may
// be filled with statute text, and optionally overrides the window itself for
// every model, which is how the overflow path is exercised without a bill the
// size of an omnibus.
func (c *Client) WithContextBudget(ratio float64, overrideTokens int) *Client {
	if ratio > 0 && ratio <= 1 {
		c.budgetRatio = ratio
	}
	if overrideTokens > 0 {
		c.contextTokens = overrideTokens
	}
	return c
}

// Enabled reports whether an API key is configured.
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

func (c *Client) logf(format string, args ...any) {
	if c.logger == nil {
		return
	}
	c.logger.Printf(format, args...)
}

const ideologySystemPrompt = `You are a nonpartisan political scientist scoring the ideological direction of United States legislation from its statutory text.

Score the bill on a scale from -1.0 to +1.0:
-1.0 = strongly progressive/liberal, 0.0 = genuinely centrist or bipartisan, +1.0 = strongly conservative.

Judge the policy content of the text, not the sponsor's party.

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
	text := clip(bill.FullText, ideologyTextChars)
	if text == "" {
		return 0, fmt.Errorf("%w: %s", ErrNoStatuteText, bill.ID)
	}
	fmt.Fprintf(&b, "\nSTATUTE TEXT:\n%s\n", text)

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
				// 2s, 4s, 8s: long enough for a rate limit window to pass.
				delay := time.Duration(int64(1)<<attempt) * 2 * time.Second
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

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
