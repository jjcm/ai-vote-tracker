package rollcall

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// houseIndexPage is the Clerk's index markup: a plain HTML table with one row
// per roll call, and no year anywhere in the row itself.
func houseIndexPage(rows ...string) string {
	return `<HTML><BODY><TABLE BORDER="1"><TR>
<TH><FONT FACE="Arial" SIZE="-1">Roll</FONT></TH>
<TH>Date</TH><TH>Issue</TH><TH>Question</TH><TH>Result</TH><TH>Title/Description</TH></TR>
` + strings.Join(rows, "\n") + `</TABLE></BODY></HTML>`
}

func houseIndexRow(roll int, date, issue, question, title string) string {
	// The Clerk links the issue column at congress.gov, which is where the
	// congress number in a row comes from.
	issueCell := issue
	if number, ok := strings.CutPrefix(issue, "H R "); ok {
		issueCell = fmt.Sprintf(`<A HREF="https://www.congress.gov/bill/119th-congress/house-bill/%s">%s</A>`, number, issue)
	}
	return fmt.Sprintf(`<TR><TD><A HREF="http://clerk.house.gov/cgi-bin/vote.asp?year=2026&rollnumber=%d">%d</A></TD>
<TD><FONT FACE="Arial" SIZE="-1">%s</FONT></TD>
<TD><FONT FACE="Arial" SIZE="-1">%s</FONT></TD>
<TD><FONT FACE="Arial" SIZE="-1">%s</FONT></TD>
<TD ALIGN="CENTER"><FONT FACE="Arial" SIZE="-1">P</FONT></TD>
<TD><FONT FACE="Arial" SIZE="-1">%s</FONT></TD></TR>`, roll, roll, date, issueCell, question, title)
}

const houseRollXML = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE rollcall-vote PUBLIC "-//US Congress//DTDs/vote v1.0 20031119 //EN" "../vote.dtd">
<rollcall-vote>
<vote-metadata><rollcall-num>283</rollcall-num><vote-type>YEA-AND-NAY</vote-type>
<vote-result>Passed</vote-result><action-date>23-Jul-2026</action-date></vote-metadata>
<vote-data>
<recorded-vote><legislator name-id="O000172" sort-field="Ocasio-Cortez" party="D" state="NY" role="legislator">Ocasio-Cortez</legislator><vote>Nay</vote></recorded-vote>
<recorded-vote><legislator name-id="M001184" sort-field="Massie" party="R" state="KY" role="legislator">Massie</legislator><vote>Aye</vote></recorded-vote>
<recorded-vote><legislator name-id="K000389" sort-field="Khanna" party="D" state="CA" role="legislator">Khanna</legislator><vote>Not Voting</vote></recorded-vote>
</vote-data></rollcall-vote>`

const quorumCallXML = `<?xml version="1.0" encoding="UTF-8"?>
<rollcall-vote><vote-metadata><vote-type>QUORUM</vote-type></vote-metadata>
<vote-data><recorded-vote><legislator name-id="M001184" party="R" state="KY">Massie</legislator><vote>Present</vote></recorded-vote></vote-data></rollcall-vote>`

const senateMenuXML = `<?xml version="1.0" encoding="UTF-8"?><vote_summary>
  <congress>119</congress><session>2</session><congress_year>2026</congress_year>
  <votes>
    <vote><vote_number>00228</vote_number><vote_date>08-Aug</vote_date><issue>H.R. 6500</issue>
      <question>On Passage of the Bill
         </question><result>Passed</result><title>H.R. 6500, as amended</title></vote>
    <vote><vote_number>00227</vote_number><vote_date>08-Aug</vote_date><issue>H.R. 6500</issue>
      <question>On the Cloture Motion</question><result>Agreed to</result><title>Cloture</title></vote>
    <vote><vote_number>00226</vote_number><vote_date>07-Aug</vote_date><issue>PN1078</issue>
      <question>On the Nomination</question><result>Confirmed</result><title>A nominee</title></vote>
    <vote><vote_number>00163</vote_number><vote_date>05-Jun</vote_date><issue>S. 2</issue>
      <question>On Passage of the Bill</question><result>Passed</result><title>S. 2, As Amended</title></vote>
    <vote><vote_number>00140</vote_number><vote_date>12-May</vote_date><issue>S. 900</issue>
      <question>On Passage of the Bill</question><result>Passed</result><title>Before the window</title></vote>
  </votes></vote_summary>`

const senateRollXML = `<?xml version="1.0" encoding="UTF-8"?><roll_call_vote>
  <congress>119</congress><session>2</session><vote_number>228</vote_number>
  <members>
    <member><member_full>Booker (D-NJ)</member_full><last_name>Booker</last_name><first_name>Cory</first_name>
      <party>D</party><state>NJ</state><vote_cast>Yea</vote_cast><lis_member_id>S370</lis_member_id></member>
    <member><member_full>Scott (R-FL)</member_full><last_name>Scott</last_name><first_name>Rick</first_name>
      <party>R</party><state>FL</state><vote_cast>Nay</vote_cast><lis_member_id>S404</lis_member_id></member>
    <member><member_full>Paul (R-KY)</member_full><last_name>Paul</last_name><first_name>Rand</first_name>
      <party>R</party><state>KY</state><vote_cast>Not Voting</vote_cast><lis_member_id>S348</lis_member_id></member>
  </members></roll_call_vote>`

type stub struct {
	mu     sync.Mutex
	agents map[string]string
}

// newStub serves the House Clerk and Senate document roots the client reads.
func newStub(t *testing.T) (*Client, *stub) {
	t.Helper()
	s := &stub{agents: map[string]string{}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.agents[r.URL.Path] = r.Header.Get("User-Agent")
		s.mu.Unlock()

		switch r.URL.Path {
		// The Clerk paginates a hundred roll calls to a page, and the first
		// page past the end is a 404.
		case "/house/2026/ROLL_000.asp":
			fmt.Fprint(w, houseIndexPage(
				houseIndexRow(99, "12-May", "H R 8646", "On Passage", "Before the window"),
			))
		case "/house/2026/ROLL_100.asp":
			fmt.Fprint(w, houseIndexPage())
		case "/house/2026/ROLL_200.asp":
			fmt.Fprint(w, houseIndexPage(
				houseIndexRow(283, "23-Jul", "H R 8884", "On Passage", "Removing Barriers to Work Act"),
				houseIndexRow(282, "23-Jul", "H CON RES 89", "On Agreeing to the Resolution", "A concurrent resolution"),
				houseIndexRow(281, "22-Jul", "H R 7008", "On Motion to Recommit", "Stop Insider Trading Act"),
				houseIndexRow(240, "14-Jul", "H R 1181", "On Motion to Suspend the Rules and Pass, as Amended", "Protecting Privacy in Purchases Act"),
			))
		case "/house/2026/roll283.xml":
			fmt.Fprint(w, houseRollXML)
		case "/house/2026/roll281.xml":
			fmt.Fprint(w, quorumCallXML)
		case "/senate/roll_call_lists/vote_menu_119_2.xml":
			fmt.Fprint(w, senateMenuXML)
		case "/senate/roll_call_votes/vote1192/vote_119_2_00228.xml":
			fmt.Fprint(w, senateRollXML)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	c := New().
		WithBaseURLs(srv.URL+"/house", srv.URL+"/senate").
		WithClock(func() time.Time { return time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC) })
	return c, s
}

func june2026() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) }

// The index keeps votes that put a bill to the chamber and drops the
// procedure, the resolutions and anything before the window opened.
func TestFinalPassageVotesFiltersTheIndex(t *testing.T) {
	c, _ := newStub(t)
	votes, err := c.FinalPassageVotes(context.Background(), june2026())
	if err != nil {
		t.Fatalf("FinalPassageVotes: %v", err)
	}

	var got []string
	for _, v := range votes {
		got = append(got, v.ID()+" "+v.BillID)
	}
	want := []string{
		"senate-119-2-163 s-2-119",
		"house-2026-240 hr-1181-119",
		"house-2026-283 hr-8884-119",
		"senate-119-2-228 hr-6500-119",
	}
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Errorf("votes = %v\nwant %v (oldest first)", got, want)
	}
}

func TestHouseIndexRowsCarryTheirContext(t *testing.T) {
	c, _ := newStub(t)
	votes, err := c.FinalPassageVotes(context.Background(), june2026())
	if err != nil {
		t.Fatalf("FinalPassageVotes: %v", err)
	}
	var passage Vote
	for _, v := range votes {
		if v.ID() == "house-2026-283" {
			passage = v
		}
	}
	if passage.Chamber != ChamberHouse || passage.Congress != 119 || passage.Session != 2 {
		t.Errorf("chamber/congress/session = %s/%d/%d", passage.Chamber, passage.Congress, passage.Session)
	}
	if want := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC); !passage.Date.Equal(want) {
		t.Errorf("date = %s, want %s: the row carries only a day and a month", passage.Date, want)
	}
	if passage.BillLabel != "H.R. 8884" || passage.Title != "Removing Barriers to Work Act" {
		t.Errorf("bill = %q title = %q", passage.BillLabel, passage.Title)
	}
}

func TestHousePositionsUseBioguideIDs(t *testing.T) {
	c, _ := newStub(t)
	positions, err := c.Positions(context.Background(), Vote{
		Chamber: ChamberHouse, Year: 2026, Number: 283,
	})
	if err != nil {
		t.Fatalf("Positions: %v", err)
	}
	if len(positions) != 3 {
		t.Fatalf("got %d positions, want 3", len(positions))
	}
	if positions[0].MemberID != "O000172" || positions[0].Cast != Nay {
		t.Errorf("first position = %+v", positions[0])
	}
	// The House says Aye in the Committee of the Whole and Yea on the floor;
	// both are the same side of the question.
	if positions[1].Cast != Yea {
		t.Errorf("Massie voted Aye, want it normalised to Yea, got %q", positions[1].Cast)
	}
	if positions[2].Cast != NotVoting || positions[2].Decided() {
		t.Errorf("an absence is not a position: %+v", positions[2])
	}
}

// A quorum call is not a division on a question, so it cannot be read as one.
func TestHouseQuorumCallIsRejected(t *testing.T) {
	c, _ := newStub(t)
	_, err := c.Positions(context.Background(), Vote{Chamber: ChamberHouse, Year: 2026, Number: 281})
	if err == nil || !strings.Contains(err.Error(), "not a division") {
		t.Fatalf("err = %v, want the quorum call to be refused", err)
	}
}

func TestSenatePositionsUseLISIDs(t *testing.T) {
	c, _ := newStub(t)
	positions, err := c.Positions(context.Background(), Vote{
		Chamber: ChamberSenate, Congress: 119, Session: 2, Number: 228,
	})
	if err != nil {
		t.Fatalf("Positions: %v", err)
	}
	if len(positions) != 3 {
		t.Fatalf("got %d positions, want 3", len(positions))
	}
	first := positions[0]
	if first.MemberID != "S370" || first.Surname != "Booker" || first.State != "NJ" || first.Cast != Yea {
		t.Errorf("first position = %+v", first)
	}
	// Two sitting senators are named Scott, which is why the state travels
	// with the surname.
	if positions[1].Surname != "Scott" || positions[1].State != "FL" {
		t.Errorf("second position = %+v", positions[1])
	}
}

// A session the Senate has not opened is missing, not broken, and the House
// votes for the year still come back.
func TestMissingSenateMenuIsNotFatal(t *testing.T) {
	c, _ := newStub(t)
	c.WithBaseURLs("", "http://127.0.0.1:1/senate-that-is-not-there")
	votes, err := c.FinalPassageVotes(context.Background(), june2026())
	if err != nil {
		t.Fatalf("a Senate outage should not lose the House: %v", err)
	}
	for _, v := range votes {
		if v.Chamber == ChamberSenate {
			t.Fatalf("no Senate votes were reachable, got %s", v.ID())
		}
	}
	if len(votes) != 2 {
		t.Errorf("got %d House votes, want 2", len(votes))
	}
}

// Both document roots sit behind a CDN that answers an unidentified client
// with a block page rather than a vote.
func TestEveryRequestIdentifiesItself(t *testing.T) {
	c, s := newStub(t)
	if _, err := c.FinalPassageVotes(context.Background(), june2026()); err != nil {
		t.Fatalf("FinalPassageVotes: %v", err)
	}
	if _, err := c.Positions(context.Background(), Vote{Chamber: ChamberHouse, Year: 2026, Number: 283}); err != nil {
		t.Fatalf("Positions: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.agents) < 3 {
		t.Fatalf("only %d path(s) requested: %v", len(s.agents), s.agents)
	}
	for path, agent := range s.agents {
		if agent != DefaultUserAgent {
			t.Errorf("%s went out as %q, want %q", path, agent, DefaultUserAgent)
		}
	}
}

func TestUserAgentOverride(t *testing.T) {
	const custom = "AIVoteTracker/1.0 (someone@example.com)"
	c, s := newStub(t)
	c.WithUserAgent(custom).WithUserAgent("   ")
	if _, err := c.FinalPassageVotes(context.Background(), june2026()); err != nil {
		t.Fatalf("FinalPassageVotes: %v", err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for path, agent := range s.agents {
		if agent != custom {
			t.Errorf("%s went out as %q, want a blank override to leave %q alone", path, agent, custom)
		}
	}
}

func TestIsFinalPassage(t *testing.T) {
	passage := []string{
		"On Passage",
		"On Passage of the Bill",
		"On Motion to Suspend the Rules and Pass",
		"On Motion to Suspend the Rules and Pass, as Amended",
		"On Motion to Suspend the Rules and Concur in Senate Adt to House Adt to Senate Adt",
		"On Motion to Concur in the Senate Amendment",
		"On the Motion to Concur in the House Amendment to the Senate Amendment",
		"On Overriding the Veto",
		"On Passage of the Bill\n         ",
	}
	procedure := []string{
		"On Motion to Recommit",
		"On Motion to Recommit with Instructions",
		"On Cloture on the Motion to Proceed",
		"On the Cloture Motion",
		"On the Motion to Proceed",
		"On the Amendment",
		"On the Motion to Table",
		"On the Nomination",
		"On Ordering the Previous Question",
		"On Agreeing to the Resolution",
		"On Approving the Journal",
	}
	for _, q := range passage {
		if !IsFinalPassage(q) {
			t.Errorf("%q should count as final passage", q)
		}
	}
	for _, q := range procedure {
		if IsFinalPassage(q) {
			t.Errorf("%q is procedure, not passage", q)
		}
	}
}

func TestBillIdentity(t *testing.T) {
	cases := []struct {
		issue string
		want  string
	}{
		{"H R 8884", "hr-8884-119"},
		{"H.R. 6500", "hr-6500-119"},
		{"HR 1181", "hr-1181-119"},
		{"S. 2", "s-2-119"},
		{"S 5271", "s-5271-119"},
	}
	for _, tc := range cases {
		id, _, ok := billIdentity(tc.issue, 119)
		if !ok || id != tc.want {
			t.Errorf("billIdentity(%q) = %q %v, want %q", tc.issue, id, ok, tc.want)
		}
	}
	// Only House and Senate bills are tracked, so everything else has to be
	// rejected rather than mistaken for one.
	for _, issue := range []string{"H RES 100", "H CON RES 89", "S J RES 12", "S RES 4", "PN1078", "S.Amdt. 6747", "", "QUORUM"} {
		if _, _, ok := billIdentity(issue, 119); ok {
			t.Errorf("%q is not a House or Senate bill", issue)
		}
	}
}

func TestCongressAndSessionForYear(t *testing.T) {
	cases := map[int][2]int{
		1789: {1, 1}, 1790: {1, 2},
		2025: {119, 1}, 2026: {119, 2}, 2027: {120, 1},
	}
	for year, want := range cases {
		if got := congressFor(year); got != want[0] {
			t.Errorf("congressFor(%d) = %d, want %d", year, got, want[0])
		}
		if got := sessionFor(year); got != want[1] {
			t.Errorf("sessionFor(%d) = %d, want %d", year, got, want[1])
		}
		if got := yearFor(want[0], want[1]); got != year {
			t.Errorf("yearFor(%d, %d) = %d, want %d", want[0], want[1], got, year)
		}
	}
}

func TestNormalizeCast(t *testing.T) {
	cases := map[string]string{
		"Yea": Yea, "Aye": Yea, "yes": Yea,
		"Nay": Nay, "No": Nay,
		"Present": Present, "Not Voting": NotVoting,
		"": "", "Guilty": "",
	}
	for raw, want := range cases {
		if got := NormalizeCast(raw); got != want {
			t.Errorf("NormalizeCast(%q) = %q, want %q", raw, got, want)
		}
	}
}
