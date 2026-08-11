package billtext

import (
	"fmt"
	"strings"
	"testing"
)

const twoSections = `<?xml version="1.0"?>
<bill><legis-body>
<section id="H1" section-type="section-one"><enum>1.</enum><header>Short title</header>
<text>This Act may be cited as the "Testing Act".</text>
</section>
<section id="H2"><enum>2.</enum><header>Personnel</header>
<text>The Secretary shall hire not fewer than 3,000 additional Border Patrol agents, and shall establish retention bonuses for agents in remote sectors.</text>
</section>
</legis-body></bill>`

func TestSplitKeepsSmallBillWhole(t *testing.T) {
	got := Split(twoSections, 100000)
	if len(got) != 1 {
		t.Fatalf("got %d pieces, want one: %v", len(got), labels(got))
	}
}

func TestSplitUsesSectionBoundaries(t *testing.T) {
	budget := 260
	got := Split(twoSections, budget)
	if len(got) < 2 {
		t.Fatalf("got %d piece(s), want the bill cut on its sections", len(got))
	}
	assertWithinBudget(t, got, budget)
	assertKeeps(t, got, "Short title", "Personnel", "retention bonuses")

	joined := strings.Join(labels(got), " | ")
	for _, want := range []string{"Sec. 1. Short title", "Sec. 2. Personnel"} {
		if !strings.Contains(joined, want) {
			t.Errorf("labels %q are missing %q", joined, want)
		}
	}
}

// A section quoted inside another section is amended text, not a sibling: it
// has to stay with the section that amends it.
func TestSplitIgnoresNestedQuotedSections(t *testing.T) {
	const nested = `<legis-body>
<section id="A"><enum>1.</enum><header>Amendment</header><text>Section 235 is amended to read as follows:
<quoted-block><section id="Q"><enum>235.</enum><header>Inspection</header><text>Quoted statute.</text></section></quoted-block></text>
</section>
<section id="B"><enum>2.</enum><header>Effective date</header><text>This Act takes effect on the date of enactment.</text></section>
</legis-body>`

	got := Split(nested, 300)
	if len(got) != 2 {
		t.Fatalf("got %d pieces, want one per top-level section: %v", len(got), labels(got))
	}
	for _, s := range got {
		if strings.Contains(s.Text, "Quoted statute.") && !strings.Contains(s.Text, "Amendment") {
			t.Errorf("piece %q carries quoted text detached from the section that amends it", s.Label)
		}
	}
}

// Titles are the boundary when the sections beneath them are not marked up.
func TestSplitFallsBackToLargerContainers(t *testing.T) {
	var body strings.Builder
	body.WriteString("<bill>")
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&body, `<title id="T%d"><enum>%d</enum><header>Programs</header><text>%s</text></title>`,
			i, i, strings.Repeat("Appropriated funds shall remain available until expended. ", 20))
	}
	body.WriteString("</bill>")

	budget := 2000
	got := Split(body.String(), budget)
	if len(got) < 2 {
		t.Fatalf("got %d piece(s), want the titles used as boundaries", len(got))
	}
	assertWithinBudget(t, got, budget)
	if !strings.Contains(got[0].Label, "Title") {
		t.Errorf("label = %q, want it to name the container", got[0].Label)
	}
}

func TestSplitPlainTextOnSectionHeadings(t *testing.T) {
	const plain = `SECTION 1. SHORT TITLE.
This Act may be cited as the "Plain Act".

SEC. 2. BORDER INFRASTRUCTURE.
There is authorized to be appropriated $8,400,000,000 for fiscal years 2026 through 2030 for physical barriers.

SEC. 3. REPORTING.
The Secretary shall submit an annual report to Congress on removals and case backlogs.`

	budget := 220
	got := Split(plain, budget)
	if len(got) < 2 {
		t.Fatalf("got %d piece(s), want the headings used as boundaries: %v", len(got), labels(got))
	}
	assertWithinBudget(t, got, budget)
	assertKeeps(t, got, "SHORT TITLE", "BORDER INFRASTRUCTURE", "REPORTING")
	if !strings.Contains(got[0].Label, "SECTION 1") {
		t.Errorf("label = %q, want the heading line", got[0].Label)
	}
}

// No structure at all still has to produce pieces inside the budget, because
// the alternative is a call the provider rejects outright.
func TestSplitAlwaysRespectsTheBudget(t *testing.T) {
	wall := strings.Repeat("The Secretary shall promulgate regulations. ", 400)
	for _, budget := range []int{300, 2500, 9000} {
		got := Split(wall, budget)
		if len(got) < 2 {
			t.Errorf("budget %d: got %d piece(s) for %d chars", budget, len(got), len(wall))
		}
		assertWithinBudget(t, got, budget)
	}
}

func TestSplitEmptyText(t *testing.T) {
	if got := Split("   \n ", 1000); got != nil {
		t.Errorf("Split(blank) = %v, want nil", got)
	}
}

func TestEstimateTokensRoundsUp(t *testing.T) {
	cases := map[string]int{"": 0, "a": 1, "abcd": 1, "abcde": 2}
	for in, want := range cases {
		if got := EstimateTokens(in); got != want {
			t.Errorf("EstimateTokens(%q) = %d, want %d", in, got, want)
		}
	}
}

func assertWithinBudget(t *testing.T, sections []Section, budget int) {
	t.Helper()
	for _, s := range sections {
		if len(s.Text) > budget {
			t.Errorf("piece %q is %d chars, over the %d budget", s.Label, len(s.Text), budget)
		}
	}
}

// assertKeeps checks that splitting did not drop any of the bill.
func assertKeeps(t *testing.T, sections []Section, fragments ...string) {
	t.Helper()
	var all strings.Builder
	for _, s := range sections {
		all.WriteString(s.Text)
	}
	for _, f := range fragments {
		if !strings.Contains(all.String(), f) {
			t.Errorf("splitting lost %q", f)
		}
	}
}

func labels(sections []Section) []string {
	out := make([]string, 0, len(sections))
	for _, s := range sections {
		out = append(out, s.Label)
	}
	return out
}
