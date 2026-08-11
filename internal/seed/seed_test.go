package seed

import (
	"strings"
	"testing"

	"github.com/pwnies/ai-vote-tracker/internal/billtext"
	"github.com/pwnies/ai-vote-tracker/internal/models"
)

func TestCorpusIsStatuteText(t *testing.T) {
	bills := Bills()
	if len(bills) < 12 {
		t.Fatalf("corpus has %d bills, want at least 12", len(bills))
	}
	ids := map[string]bool{}
	for _, b := range bills {
		if ids[b.ID] {
			t.Errorf("duplicate bill id %q", b.ID)
		}
		ids[b.ID] = true

		if !b.HasStatuteText() {
			t.Errorf("%s does not count as carrying statute text (source %q)", b.ID, b.TextSource)
		}
		if b.TextSource != models.TextSourceSeedXML {
			t.Errorf("%s text source = %q", b.ID, b.TextSource)
		}
		if !strings.HasPrefix(b.FullText, "<?xml") || !strings.Contains(b.FullText, "<legis-body>") {
			t.Errorf("%s is not bill XML: %.80q", b.ID, b.FullText)
		}
		// Long enough that the models are reading a statute rather than a
		// paraphrase of one.
		if len(b.FullText) < 6000 {
			t.Errorf("%s statute text is only %d chars", b.ID, len(b.FullText))
		}
		if b.Summary == "" {
			t.Errorf("%s has no summary for the bill cards", b.ID)
		}
		if strings.Contains(b.FullText, b.Summary) {
			t.Errorf("%s statute text contains the CRS-style summary", b.ID)
		}
		if b.StatusCategory == "" || b.Source != models.SourceSeed || b.TextVersion == "" {
			t.Errorf("%s metadata incomplete: status=%q source=%q version=%q",
				b.ID, b.StatusCategory, b.Source, b.TextVersion)
		}
	}
}

// The corpus has to survive the splitter: every bill divides on its own
// structure rather than being cut mid-markup.
func TestCorpusSplitsOnItsOwnStructure(t *testing.T) {
	for _, b := range Bills() {
		pieces := billtext.Split(b.FullText, 4000)
		if len(pieces) < 2 {
			t.Errorf("%s (%d chars) did not split", b.ID, len(b.FullText))
			continue
		}
		named := 0
		for _, p := range pieces {
			if len(p.Text) > 4000 {
				t.Errorf("%s piece %q is %d chars, over budget", b.ID, p.Label, len(p.Text))
			}
			if strings.Contains(p.Label, "Sec.") || strings.Contains(p.Label, "Title") {
				named++
			}
		}
		if named == 0 {
			t.Errorf("%s pieces are unnamed: the splitter found no structure", b.ID)
		}
	}
}

// The appropriations act is the long one, and it is written as many short
// account sections the way a real one is.
func TestAppropriationsBillIsTheLongOne(t *testing.T) {
	b := appropriations()
	if len(b.FullText) < 20000 {
		t.Errorf("statute text is %d chars, want a genuinely long bill", len(b.FullText))
	}
	if !strings.Contains(b.FullText, "<division") || !strings.Contains(b.FullText, "<title") {
		t.Error("appropriations text is missing its division and title structure")
	}
	if n := strings.Count(b.FullText, "<section "); n < 25 {
		t.Errorf("%d account sections, want an act written account by account", n)
	}
}

// The bill XML has to be well formed enough that no unescaped angle bracket or
// ampersand from the drafting leaks into the markup.
func TestBillXMLEscapesContent(t *testing.T) {
	for _, b := range Bills() {
		body := b.FullText
		for _, bad := range []string{" & ", "<<", "</>"} {
			if strings.Contains(body, bad) {
				t.Errorf("%s contains unescaped %q", b.ID, bad)
			}
		}
		if opens, closes := strings.Count(body, "<section "), strings.Count(body, "</section>"); opens != closes {
			t.Errorf("%s has %d <section> opens and %d closes", b.ID, opens, closes)
		}
	}
}
