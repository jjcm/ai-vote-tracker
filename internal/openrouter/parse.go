package openrouter

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// This file is the salvage layer. Models wrap their JSON in markdown fences,
// prefix it with chatter, answer in prose instead, or stop mid-string when the
// token budget runs out — and every one of those shapes still has to yield
// usable content or nothing at all, never the response envelope itself.

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
	if !ok {
		return "", false
	}
	value, _, ok := readString(rest)
	return value, ok
}

// jsonStringArrayField reads an array of strings the same forgiving way: an
// array cut off after its third entry yields those three entries.
func jsonStringArrayField(body, key string) ([]string, bool) {
	rest, ok := seekValue(body, key)
	if !ok || !strings.HasPrefix(rest, "[") {
		return nil, false
	}
	rest = rest[1:]

	var out []string
	for {
		rest = strings.TrimLeft(rest, " \t\r\n,")
		if rest == "" || rest[0] == ']' {
			break
		}
		if rest[0] != '"' {
			// A non-string entry means this is not the array we can read.
			break
		}
		value, consumed, ok := readString(rest)
		if !ok {
			break
		}
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
		if consumed >= len(rest) {
			break
		}
		rest = rest[consumed:]
	}
	return out, len(out) > 0
}

// readString unquotes the JSON string at the head of s, closing the quote
// itself if the reply was cut off mid-value. The second return is how many
// bytes of s the value occupied.
func readString(s string) (value string, consumed int, ok bool) {
	if !strings.HasPrefix(s, `"`) {
		return "", 0, false
	}
	end := -1
	for i := 1; i < len(s); i++ {
		if s[i] == '\\' {
			i++
			continue
		}
		if s[i] == '"' {
			end = i
			break
		}
	}

	quoted := s
	consumed = len(s)
	if end >= 0 {
		quoted, consumed = s[:end+1], end+1
	} else {
		// Truncated mid-string. Close the quote ourselves, dropping a dangling
		// backslash first so it cannot escape the quote we are adding.
		quoted = strings.TrimRight(quoted, "\\") + `"`
	}
	if out, err := strconv.Unquote(quoted); err == nil {
		return out, consumed, true
	}
	return strings.Trim(quoted, `"`), consumed, true
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

// firstString reads one string field from the first JSON object in a reply.
func firstString(content, key string) string {
	body, _, ok := jsonObject(content)
	if !ok {
		return ""
	}
	value, _ := jsonStringField(body, key)
	return strings.TrimSpace(value)
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

func snippet(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

func clip(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return strings.TrimSpace(s[:max]) + "…"
}
