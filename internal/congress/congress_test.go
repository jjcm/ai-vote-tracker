package congress

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// statute is long enough to pass the "is this really a bill" floor.
var statute = `<?xml version="1.0"?><bill><legis-body><section id="H1"><enum>1.</enum>` +
	`<header>Short title</header><text>` +
	strings.Repeat("This Act may be cited as the Testing Act. ", 20) +
	`</text></section></legis-body></bill>`

// stub serves the three Congress.gov endpoints the client uses: the summaries
// feed, the per-bill detail, and the text versions list, plus the congress.gov
// file downloads the last of those points at.
type stub struct {
	textVersions string // JSON body for /text

	mu     sync.Mutex
	hits   map[string]int
	agents map[string]string // path -> the User-Agent it arrived with
}

func (s *stub) record(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits[r.URL.Path]++
	s.agents[r.URL.Path] = r.Header.Get("User-Agent")
}

// seenAgents returns the user agent every request arrived with, by path.
func (s *stub) seenAgents() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.agents))
	for path, agent := range s.agents {
		out[path] = agent
	}
	return out
}

func newStub(t *testing.T, textVersions string) (*Client, *stub) {
	t.Helper()
	s := &stub{textVersions: textVersions, hits: map[string]int{}, agents: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.record(r)
		switch {
		case r.URL.Path == "/summaries":
			fmt.Fprintf(w, `{"summaries":[{"actionDate":"2025-04-02","actionDesc":"Introduced in Senate",
			  "text":"<p>%s</p>","updateDate":"2025-05-08T00:00:00Z",
			  "bill":{"congress":119,"number":"1264","originChamber":"Senate","type":"S",
			  "title":"Border Security and Asylum Reform Act of 2025"}}]}`,
				strings.Repeat("This bill strengthens border security measures. ", 5))
		case strings.HasSuffix(r.URL.Path, "/text"):
			fmt.Fprint(w, s.textVersions)
		case strings.HasSuffix(r.URL.Path, ".xml"):
			fmt.Fprint(w, statute)
		case strings.HasSuffix(r.URL.Path, ".htm"):
			fmt.Fprintf(w, "<html><body><pre>%s</pre></body></html>", strings.Repeat("SEC. 2. PERSONNEL. The Secretary shall hire officers. ", 20))
		case strings.HasSuffix(r.URL.Path, ".pdf"):
			http.Error(w, "not text", http.StatusNotFound)
		default:
			fmt.Fprint(w, `{"bill":{"introducedDate":"2025-04-02","latestAction":{"text":"Read twice and referred to committee"},
			  "policyArea":{"name":"Immigration"},"sponsors":[{"fullName":"Sen. Marshall, Roger [R-KS]","party":"R"}]}}`)
		}
	}))
	t.Cleanup(srv.Close)

	c := New(srv.URL, "test-key")
	// The text version URLs are absolute in the real feed; point them at the
	// stub by templating its address into the fixture.
	s.textVersions = strings.ReplaceAll(s.textVersions, "{{base}}", srv.URL)
	return c, s
}

// The newest version that offers XML is the one that gets read.
func TestRecentBillsPrefersTheLatestFormattedXML(t *testing.T) {
	c, _ := newStub(t, `{"textVersions":[
	  {"type":"Introduced in Senate","date":"2025-04-02T04:00:00Z","formats":[
	    {"type":"Formatted Text","url":"{{base}}/119/bills/s1264/BILLS-119s1264is.htm"},
	    {"type":"Formatted XML","url":"{{base}}/119/bills/s1264/BILLS-119s1264is.xml"}]},
	  {"type":"Reported in Senate","date":"2025-05-01T04:00:00Z","formats":[
	    {"type":"PDF","url":"{{base}}/119/bills/s1264/BILLS-119s1264rs.pdf"},
	    {"type":"Formatted XML","url":"{{base}}/119/bills/s1264/BILLS-119s1264rs.xml"}]}]}`)

	bills, err := c.RecentBills(context.Background(), 1)
	if err != nil {
		t.Fatalf("RecentBills: %v", err)
	}
	if len(bills) != 1 {
		t.Fatalf("got %d bills, want 1", len(bills))
	}

	b := bills[0]
	if b.TextSource != models.TextSourceCongressXML || b.TextFormat != formatXML {
		t.Errorf("text source/format = %q/%q, want the XML print", b.TextSource, b.TextFormat)
	}
	if b.TextVersion != "Reported in Senate" {
		t.Errorf("text version = %q, want the newest one that has XML", b.TextVersion)
	}
	if !strings.HasSuffix(b.TextURL, "BILLS-119s1264rs.xml") {
		t.Errorf("text url = %q", b.TextURL)
	}
	if !strings.Contains(b.FullText, "<section") {
		t.Errorf("full text is not the bill XML: %q", clip(b.FullText))
	}
	if !b.HasStatuteText() {
		t.Error("the bill should count as carrying statute text")
	}
	// The CRS summary survives for the bill cards, but it is not the text the
	// models read.
	if b.Summary == "" || strings.Contains(b.FullText, "This bill strengthens") {
		t.Errorf("summary %q leaked into the statute text", b.Summary)
	}
	if b.PolicyArea != "Immigration" || b.Sponsor == "" || b.Status == "" {
		t.Errorf("bill detail was not merged in: %+v", b)
	}
}

// PDF is not machine-readable text here, so the HTML print is the fallback.
func TestRecentBillsFallsBackToFormattedText(t *testing.T) {
	var logs strings.Builder
	c, _ := newStub(t, `{"textVersions":[
	  {"type":"Introduced in Senate","date":"2025-04-02T04:00:00Z","formats":[
	    {"type":"PDF","url":"{{base}}/119/bills/s1264/BILLS-119s1264is.pdf"},
	    {"type":"Formatted Text","url":"{{base}}/119/bills/s1264/BILLS-119s1264is.htm"}]}]}`)
	c.WithLogger(log.New(&logs, "", 0))

	bills, err := c.RecentBills(context.Background(), 1)
	if err != nil {
		t.Fatalf("RecentBills: %v", err)
	}
	b := bills[0]
	if b.TextSource != models.TextSourceCongressHTML || b.TextFormat != formatHTML {
		t.Errorf("text source/format = %q/%q, want the HTML print", b.TextSource, b.TextFormat)
	}
	if strings.Contains(b.FullText, "<pre>") {
		t.Errorf("HTML markup was not stripped: %q", clip(b.FullText))
	}
	if !strings.Contains(b.FullText, "SEC. 2. PERSONNEL.") {
		t.Errorf("full text is missing the statute: %q", clip(b.FullText))
	}
	if !strings.Contains(logs.String(), "no XML text version") {
		t.Errorf("the fallback was not logged: %q", logs.String())
	}
}

// A bill whose text Congress has not published yet is skipped: there is
// nothing lawful to read, and its summary is not a substitute.
func TestRecentBillsSkipsBillsWithoutText(t *testing.T) {
	var logs strings.Builder
	c, _ := newStub(t, `{"textVersions":[]}`)
	c.WithLogger(log.New(&logs, "", 0))

	if _, err := c.RecentBills(context.Background(), 1); err == nil {
		t.Fatal("a feed of bills without text must not yield bills")
	}
	if !strings.Contains(logs.String(), "no readable text version yet") {
		t.Errorf("the skip was not logged: %q", logs.String())
	}
}

// PDF-only is the same as no text: nothing here can be read as statute.
func TestRecentBillsSkipsPDFOnlyBills(t *testing.T) {
	c, _ := newStub(t, `{"textVersions":[{"type":"Introduced in Senate","date":"2025-04-02T04:00:00Z",
	  "formats":[{"type":"PDF","url":"{{base}}/119/bills/s1264/BILLS-119s1264is.pdf"}]}]}`)
	if _, err := c.RecentBills(context.Background(), 1); err == nil {
		t.Fatal("a PDF-only bill must be skipped")
	}
}

// A format record that names only the file extension still counts as XML.
func TestRecentBillsReadsUnlabelledXML(t *testing.T) {
	c, _ := newStub(t, `{"textVersions":[{"type":"Enrolled Bill","date":"2025-06-01T04:00:00Z",
	  "formats":[{"url":"{{base}}/119/bills/s1264/BILLS-119s1264enr.xml"}]}]}`)
	bills, err := c.RecentBills(context.Background(), 1)
	if err != nil {
		t.Fatalf("RecentBills: %v", err)
	}
	if bills[0].TextSource != models.TextSourceCongressXML {
		t.Errorf("text source = %q, want the XML print", bills[0].TextSource)
	}
}

func TestRecentBillsWithoutKey(t *testing.T) {
	if _, err := New("https://api.congress.gov/v3", "").RecentBills(context.Background(), 1); err == nil {
		t.Fatal("expected an error without an API key")
	}
}

func clip(s string) string {
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

// Congress.gov is behind a CDN that answers an unidentified request with a
// block page instead of a bill: 403 from www.congress.gov, and a Cloudflare
// 1010 from api.congress.gov on some networks. Every request has to say who it
// is — the JSON calls and the text downloads alike.
func TestEveryRequestIdentifiesItself(t *testing.T) {
	c, s := newStub(t, `{"textVersions":[{"type":"Introduced in Senate","date":"2025-04-02T04:00:00Z",
	  "formats":[{"type":"Formatted XML","url":"{{base}}/119/bills/s1264/BILLS-119s1264is.xml"}]}]}`)

	if _, err := c.RecentBills(context.Background(), 1); err != nil {
		t.Fatalf("RecentBills: %v", err)
	}

	agents := s.seenAgents()
	if len(agents) < 4 {
		t.Fatalf("only %d path(s) were requested: %v", len(agents), agents)
	}
	for path, agent := range agents {
		if agent == "" {
			t.Errorf("%s went out with no user agent", path)
		}
		if strings.HasPrefix(agent, "Go-http-client") {
			t.Errorf("%s went out with the default library user agent %q", path, agent)
		}
		if agent != DefaultUserAgent {
			t.Errorf("%s user agent = %q, want %q", path, agent, DefaultUserAgent)
		}
	}
	// Named explicitly so a refactor cannot quietly drop the download, which is
	// the request that fails hardest without a user agent.
	for _, want := range []string{"/summaries", "/119/bills/s1264/BILLS-119s1264is.xml"} {
		if _, ok := agents[want]; !ok {
			t.Errorf("%s was never requested: %v", want, agents)
		}
	}
}

// An operator adding contact details replaces the agent everywhere.
func TestUserAgentOverrideAppliesToDownloadsToo(t *testing.T) {
	const custom = "AIVoteTracker/1.0 (someone@example.com)"
	c, s := newStub(t, `{"textVersions":[{"type":"Introduced in Senate","date":"2025-04-02T04:00:00Z",
	  "formats":[{"type":"Formatted XML","url":"{{base}}/119/bills/s1264/BILLS-119s1264is.xml"}]}]}`)
	c.WithUserAgent(custom)

	if _, err := c.RecentBills(context.Background(), 1); err != nil {
		t.Fatalf("RecentBills: %v", err)
	}
	for path, agent := range s.seenAgents() {
		if agent != custom {
			t.Errorf("%s user agent = %q, want %q", path, agent, custom)
		}
	}
}

// An unset CONGRESS_USER_AGENT must not become an empty header, which is the
// one thing the CDN refuses outright.
func TestBlankUserAgentKeepsTheDefault(t *testing.T) {
	c, s := newStub(t, `{"textVersions":[{"type":"Introduced in Senate","date":"2025-04-02T04:00:00Z",
	  "formats":[{"type":"Formatted XML","url":"{{base}}/119/bills/s1264/BILLS-119s1264is.xml"}]}]}`)
	c.WithUserAgent("   ")

	if _, err := c.RecentBills(context.Background(), 1); err != nil {
		t.Fatalf("RecentBills: %v", err)
	}
	for path, agent := range s.seenAgents() {
		if agent != DefaultUserAgent {
			t.Errorf("%s user agent = %q, want the default to survive a blank override", path, agent)
		}
	}
}
