// Package billtext works on the statute text of a bill: estimating how much of
// a model's context window it will occupy, and cutting it into pieces small
// enough to read when it will not fit.
//
// Splitting prefers the structural boundaries of the Congress.gov bill XML —
// a section, then the larger containers around it — so that each piece is a
// self-contained unit of law rather than an arbitrary slice of markup. Plain
// text falls back to "SEC. n." headings, and text with no headings at all to
// paragraph boundaries.
package billtext

import (
	"fmt"
	"regexp"
	"strings"
)

// CharsPerToken is a deliberately conservative characters-per-token ratio.
// Bill XML tokenizes worse than prose because of the tag soup and the hex
// element ids, so assuming four characters per token overstates the budget a
// text will need rather than understating it.
const CharsPerToken = 4

// EstimateTokens approximates how many tokens a string will occupy.
func EstimateTokens(s string) int {
	return (len(s) + CharsPerToken - 1) / CharsPerToken
}

// Section is one readable piece of a bill.
type Section struct {
	// Label names the piece for the model, e.g. "Sec. 4. Expedited removal".
	Label string
	Text  string
}

// Tags are the bill XML containers tried as split boundaries, finest first: a
// section is the natural unit of a statute, and the larger containers only
// matter for bills that group sections under them without marking the sections
// themselves.
var Tags = []string{"section", "title", "subtitle", "part", "division"}

// Split cuts text into pieces of at most maxChars characters each, preferring
// structural boundaries. A single structural unit larger than maxChars is cut
// further on line boundaries, so no piece ever exceeds the budget.
func Split(text string, maxChars int) []Section {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	// A floor keeps a misconfigured budget from shredding a bill into
	// fragments too small to summarize.
	if maxChars < 200 {
		maxChars = 200
	}
	units := structuralUnits(text)
	if len(units) == 0 {
		units = headingUnits(text)
	}
	if len(units) == 0 {
		units = []Section{{Label: "Full text", Text: text}}
	}
	return pack(units, maxChars)
}

// structuralUnits splits bill XML on the first container tag that actually
// divides it. Nested occurrences are ignored: a section quoted inside another
// section is amended text, not a sibling.
func structuralUnits(text string) []Section {
	for _, tag := range Tags {
		spans := topLevelSpans(text, tag)
		if len(spans) < 2 {
			continue
		}
		units := make([]Section, 0, len(spans)+1)
		if preamble := strings.TrimSpace(text[:spans[0].start]); len(preamble) > 0 {
			units = append(units, Section{Label: "Front matter", Text: preamble})
		}
		for i, s := range spans {
			body := text[s.start:s.end]
			units = append(units, Section{Label: xmlLabel(tag, body, i), Text: body})
		}
		if tail := strings.TrimSpace(text[spans[len(spans)-1].end:]); len(tail) > 0 {
			units = append(units, Section{Label: "Back matter", Text: tail})
		}
		return units
	}
	return nil
}

type span struct{ start, end int }

// topLevelSpans finds the outermost <tag>…</tag> ranges in text.
func topLevelSpans(text, tag string) []span {
	openTag, closeTag := "<"+tag, "</"+tag
	var out []span
	depth, start, i := 0, 0, 0
	for i < len(text) {
		oi := indexTag(text, openTag, i)
		ci := indexTag(text, closeTag, i)
		if oi < 0 && ci < 0 {
			break
		}
		if oi >= 0 && (ci < 0 || oi < ci) {
			end, selfClosing := tagEnd(text, oi)
			if selfClosing {
				i = end
				continue
			}
			if depth == 0 {
				start = oi
			}
			depth++
			i = end
			continue
		}
		end, _ := tagEnd(text, ci)
		if depth > 0 {
			depth--
			if depth == 0 {
				out = append(out, span{start, end})
			}
		}
		i = end
	}
	return out
}

// indexTag finds the next occurrence of an element name, so that <sections>
// does not match a search for <section>.
func indexTag(text, tag string, from int) int {
	for i := from; ; {
		at := strings.Index(text[i:], tag)
		if at < 0 {
			return -1
		}
		at += i
		next := at + len(tag)
		if next >= len(text) {
			return -1
		}
		switch text[next] {
		case '>', '/', ' ', '\t', '\r', '\n':
			return at
		}
		i = next
	}
}

// tagEnd returns the offset just past the '>' that closes the tag starting at
// from, and whether that tag closes itself.
func tagEnd(text string, from int) (int, bool) {
	gt := strings.IndexByte(text[from:], '>')
	if gt < 0 {
		return len(text), false
	}
	end := from + gt + 1
	return end, gt > 0 && text[from+gt-1] == '/'
}

var (
	enumRE   = regexp.MustCompile(`(?s)<enum[^>]*>(.*?)</enum>`)
	headerRE = regexp.MustCompile(`(?s)<header[^>]*>(.*?)</header>`)
	tagRE    = regexp.MustCompile(`(?s)<[^>]*>`)
)

// xmlLabel builds a human label from the <enum> and <header> a bill container
// normally opens with.
func xmlLabel(tag, body string, index int) string {
	var parts []string
	if m := enumRE.FindStringSubmatch(body); m != nil {
		parts = append(parts, collapse(tagRE.ReplaceAllString(m[1], "")))
	}
	if m := headerRE.FindStringSubmatch(body); m != nil {
		parts = append(parts, collapse(tagRE.ReplaceAllString(m[1], "")))
	}
	label := strings.TrimSpace(strings.Join(parts, " "))
	if label == "" {
		return fmt.Sprintf("%s %d", capitalize(tag), index+1)
	}
	prefix := "Sec."
	if tag != "section" {
		prefix = capitalize(tag)
	}
	if strings.HasPrefix(strings.ToLower(label), strings.ToLower(prefix)) {
		return clip(label, 90)
	}
	return clip(prefix+" "+label, 90)
}

// headingHeadRE matches the headings a plain-text bill is divided by.
var headingHeadRE = regexp.MustCompile(`(?mi)^[ \t]*((?:SEC(?:TION)?\.?[ \t]+[0-9]+[A-Z]?\.?)|(?:TITLE[ \t]+[IVXLC]+)|(?:DIVISION[ \t]+[A-Z])|(?:SUBTITLE[ \t]+[A-Z]))`)

// headingUnits splits plain statute text on its section headings, and failing
// that on blank lines.
func headingUnits(text string) []Section {
	at := headingHeadRE.FindAllStringIndex(text, -1)
	if len(at) >= 2 {
		units := make([]Section, 0, len(at)+1)
		if preamble := strings.TrimSpace(text[:at[0][0]]); preamble != "" {
			units = append(units, Section{Label: "Front matter", Text: preamble})
		}
		for i, m := range at {
			end := len(text)
			if i+1 < len(at) {
				end = at[i+1][0]
			}
			body := strings.TrimSpace(text[m[0]:end])
			units = append(units, Section{Label: textLabel(body), Text: body})
		}
		return units
	}

	var units []Section
	for i, block := range strings.Split(text, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		units = append(units, Section{Label: fmt.Sprintf("Paragraph %d", i+1), Text: block})
	}
	if len(units) < 2 {
		return nil
	}
	return units
}

func textLabel(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return clip(collapse(line), 90)
		}
	}
	return "Section"
}

// pack fills chunks up to maxChars, keeping units whole where it can and
// cutting an oversized unit on line boundaries where it cannot.
func pack(units []Section, maxChars int) []Section {
	var out []Section
	var cur []Section
	curLen := 0

	flush := func() {
		if len(cur) == 0 {
			return
		}
		out = append(out, join(cur))
		cur, curLen = nil, 0
	}

	for _, u := range units {
		if len(u.Text) > maxChars {
			flush()
			for i, piece := range cutLines(u.Text, maxChars) {
				out = append(out, Section{
					Label: fmt.Sprintf("%s (part %d)", u.Label, i+1),
					Text:  piece,
				})
			}
			continue
		}
		if curLen+len(u.Text) > maxChars {
			flush()
		}
		cur = append(cur, u)
		curLen += len(u.Text) + 2
	}
	flush()
	return out
}

func join(units []Section) Section {
	if len(units) == 1 {
		return units[0]
	}
	bodies := make([]string, 0, len(units))
	for _, u := range units {
		bodies = append(bodies, u.Text)
	}
	return Section{
		Label: units[0].Label + " through " + units[len(units)-1].Label,
		Text:  strings.Join(bodies, "\n\n"),
	}
}

// cutLines slices text into pieces of at most maxChars, breaking on the last
// line boundary that fits so a piece never ends mid-line.
func cutLines(text string, maxChars int) []string {
	var out []string
	for len(text) > maxChars {
		cut := strings.LastIndexByte(text[:maxChars], '\n')
		if cut < maxChars/2 {
			cut = maxChars
		}
		out = append(out, strings.TrimSpace(text[:cut]))
		text = text[cut:]
	}
	if rest := strings.TrimSpace(text); rest != "" {
		out = append(out, rest)
	}
	return out
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func collapse(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "\u00a0", " ")), " ")
}

func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return strings.TrimRight(s[:max], " ,;:") + "…"
}
