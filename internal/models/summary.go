package models

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SummaryWithoutTitleEcho drops a leading restatement of the bill's title from
// its summary. The CRS summaries Congress.gov publishes open with the short
// title as its own paragraph, so once that markup is flattened every summary
// begins by repeating the headline printed directly above it.
//
// Only an echo is removed. If the summary's opening carries anything the title
// does not, or if removing the echo would leave next to nothing, the summary is
// returned untouched.
func SummaryWithoutTitleEcho(title, summary string) string {
	summary = strings.TrimSpace(summary)
	variants := titleVariants(title)
	if summary == "" || len(variants) == 0 {
		return summary
	}

	limit := 0
	for v := range variants {
		if n := len(strings.Fields(v)); n > limit {
			limit = n
		}
	}
	// Room for the boilerplate a summary may put in front of the title, and no
	// more: an echo is the opening words or it is not an echo.
	limit += len(strings.Fields(citationPrefix))

	// The longest echo wins. A title whose year has been dropped also matches
	// the words before that year, and cutting there would leave the year behind.
	trimmed := summary
	words := make([]string, 0, limit)
	for _, span := range summaryWordRE.FindAllStringIndex(summary, limit) {
		words = append(words, strings.ToLower(summary[span[0]:span[1]]))
		if !matchesTitle(strings.Join(words, " "), variants) {
			continue
		}
		tail := summary[span[1]:]
		rest := strings.TrimSpace(tail)
		if !closesClause(tail, rest) {
			continue
		}
		rest = strings.TrimSpace(strings.TrimLeft(rest, ".:;,-\u2013\u2014"))
		// A summary that was nothing but its own title has no better form to
		// offer, so leave it as it was rather than blanking the description.
		if utf8.RuneCountInString(rest) >= minSummaryRunes {
			trimmed = rest
		}
	}
	return trimmed
}

// minSummaryRunes is the shortest remainder worth substituting for the original.
const minSummaryRunes = 24

// citationPrefix is the longest boilerplate matchesTitle will step over, and
// sets how far past the title's own length an echo may start.
const citationPrefix = "this bill may be cited as the"

var (
	summaryWordRE = regexp.MustCompile(`[A-Za-z0-9]+`)
	// Congress is inconsistent about whether a short title carries its year:
	// "Border Security Act of 2025", "Border Security Act, 2025" and "Border
	// Security Act" all have to match one another.
	trailingYearRE = regexp.MustCompile(`\s+(?:of\s+)?(?:19|20)\d{2}$`)
	// Wording a summary may place in front of the title before restating it.
	citationPrefixRE = regexp.MustCompile(`^(?:(?:this|the) (?:bill|act|measure|resolution) (?:is|may be cited as)\s+)?(?:the\s+)?`)
)

// titleVariants is the set of normalized forms a summary might echo the title
// in. A title of fewer than three words is not offered: short ones are matched
// by accident too easily to be worth stripping.
func titleVariants(title string) map[string]bool {
	base := normalizeWords(title)
	if len(strings.Fields(base)) < 3 {
		return nil
	}
	variants := map[string]bool{base: true}
	if shorter := trailingYearRE.ReplaceAllString(base, ""); shorter != base {
		variants[shorter] = true
	}
	return variants
}

// matchesTitle reports whether candidate says only what the title says.
func matchesTitle(candidate string, variants map[string]bool) bool {
	candidate = citationPrefixRE.ReplaceAllString(candidate, "")
	if candidate == "" {
		return false
	}
	return variants[candidate] || variants[trailingYearRE.ReplaceAllString(candidate, "")]
}

// closesClause reports whether the echo ends where it stopped rather than
// running on into the sentence. "Border Security Act of 2025 would create an
// office" is the title used as a subject: cutting it leaves a verb with nothing
// to attach to, so it is left alone.
func closesClause(after, rest string) bool {
	for _, mark := range []string{".", "\n", ":", ";", "-", "\u2013", "\u2014"} {
		if strings.HasPrefix(strings.TrimLeft(after, " \t"), mark) {
			return true
		}
	}
	first, _ := utf8.DecodeRuneInString(rest)
	return unicode.IsUpper(first)
}

func normalizeWords(s string) string {
	return strings.Join(summaryWordRE.FindAllString(strings.ToLower(s), -1), " ")
}
