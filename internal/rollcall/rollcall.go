// Package rollcall reads how individual members of Congress voted on the floor.
//
// # Why not the Congress.gov API
//
// The rest of this project talks to api.congress.gov, and that is where you
// would expect member positions to come from too. They are not there for both
// chambers. Congress.gov exposes House recorded votes through a beta
// /v3/house-vote endpoint and has no Senate equivalent at all, so a
// Congress.gov-only implementation could compare a model against Ocasio-Cortez
// and Massie but not against a single senator — and the Senate is where almost
// every 2028 potential who can vote actually sits.
//
// Both chambers do publish complete per-member roll calls themselves, as XML,
// with no key and no beta caveat:
//
//   - the Clerk of the House at clerk.house.gov/evs, which stamps each
//     legislator with their Bioguide ID;
//   - the Senate at senate.gov/legislative/LIS, which identifies senators by
//     LIS member ID.
//
// These are the sources the Congress.gov vote endpoints are themselves built
// from, so this package reads them directly. Both sit behind a CDN that
// answers an unidentified client with a block page, so every request is
// stamped with a user agent by the transport.
package rollcall

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// DefaultUserAgent identifies this client to the House and Senate. Sending
// none, or a default library agent, draws a block page rather than a vote.
const DefaultUserAgent = "AIVoteTracker/1.0 (+https://github.com/pwnies/ai-vote-tracker)"

// Default document roots. Both are public and unauthenticated.
const (
	DefaultHouseBase  = "https://clerk.house.gov/evs"
	DefaultSenateBase = "https://www.senate.gov/legislative/LIS"
)

// Chambers, matching models.Chamber*.
const (
	ChamberHouse  = "House"
	ChamberSenate = "Senate"
)

// Positions a member can be recorded in. Present and Not Voting are kept so a
// caller can tell "did not take a side" from "was never asked".
const (
	Yea       = "Yea"
	Nay       = "Nay"
	Present   = "Present"
	NotVoting = "Not Voting"
)

// maxBody is the read ceiling. A full House roll call is a few hundred
// kilobytes of XML; nothing here is close to this.
const maxBody = 16 << 20

// maxIndexPages bounds the walk through the Clerk's paginated index. Each page
// holds a hundred roll calls, so this is several times the busiest year on
// record — it exists so a document root that stops answering 404 cannot spin.
const maxIndexPages = 20

// Vote is one roll call: the question, what it was on, and where the
// per-member tally lives.
type Vote struct {
	Chamber  string
	Congress int
	Session  int
	Year     int
	// Number is the roll call number within the chamber's year (House) or
	// within the congress and session (Senate).
	Number int
	Date   time.Time
	// BillID is the corpus identifier of the bill voted on, in the same
	// "hr-8884-119" shape internal/congress mints. Empty when the roll call was
	// not on a House or Senate bill.
	BillID    string
	BillLabel string
	Question  string
	Result    string
	Title     string
	URL       string
}

// ID uniquely names a roll call across chambers and sessions.
func (v Vote) ID() string {
	if v.Chamber == ChamberSenate {
		return fmt.Sprintf("senate-%d-%d-%03d", v.Congress, v.Session, v.Number)
	}
	return fmt.Sprintf("house-%d-%03d", v.Year, v.Number)
}

// Position is one member's recorded position on one roll call.
type Position struct {
	// MemberID is whichever identifier that chamber publishes: a Bioguide ID
	// from the House Clerk, an LIS member ID from the Senate.
	MemberID string
	Surname  string
	State    string
	Party    string
	Cast     string
}

// Decided reports whether the member took a binary side, which is the only
// case an agreement rate can be computed from.
func (p Position) Decided() bool { return p.Cast == Yea || p.Cast == Nay }

// Client reads roll calls from the House Clerk and the Senate.
type Client struct {
	houseBase  string
	senateBase string
	http       *http.Client
	logger     *log.Logger
	// now is the clock, so tests can pin the window.
	now func() time.Time
}

// New builds a client against the live House and Senate document roots.
func New() *Client {
	return &Client{
		houseBase:  DefaultHouseBase,
		senateBase: DefaultSenateBase,
		http: &http.Client{
			Timeout:   60 * time.Second,
			Transport: identify(DefaultUserAgent, nil),
		},
		now: time.Now,
	}
}

// WithBaseURLs points the client at different document roots, which is how the
// tests serve fixtures.
func (c *Client) WithBaseURLs(house, senate string) *Client {
	if house != "" {
		c.houseBase = strings.TrimRight(house, "/")
	}
	if senate != "" {
		c.senateBase = strings.TrimRight(senate, "/")
	}
	return c
}

// WithUserAgent replaces the agent sent with every request. A blank agent is
// ignored rather than sent: an unset variable must not become a request the
// CDN refuses.
func (c *Client) WithUserAgent(agent string) *Client {
	if agent = strings.TrimSpace(agent); agent != "" {
		c.http.Transport = identify(agent, nil)
	}
	return c
}

// WithLogger records which indexes were read and which rolls were skipped.
func (c *Client) WithLogger(logger *log.Logger) *Client {
	c.logger = logger
	return c
}

// WithClock pins the upper end of the search window, for tests.
func (c *Client) WithClock(now func() time.Time) *Client {
	if now != nil {
		c.now = now
	}
	return c
}

func (c *Client) logf(format string, args ...any) {
	if c.logger != nil {
		c.logger.Printf(format, args...)
	}
}

type identifier struct {
	agent string
	base  http.RoundTripper
}

func identify(agent string, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return identifier{agent: agent, base: base}
}

func (t identifier) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.Header.Set("User-Agent", t.agent)
	return t.base.RoundTrip(clone)
}

// errNotFound marks a document root that ran out of pages, which is how both
// indexes say "that was the last roll call".
var errNotFound = errors.New("rollcall: no such document")

func (c *Client) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rollcall: GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", errNotFound, url)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rollcall: GET %s returned HTTP %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, fmt.Errorf("rollcall: read %s: %w", url, err)
	}
	return body, nil
}

// FinalPassageVotes lists every House and Senate roll call on a bill's final
// passage taken on or after since. Procedural votes — cloture, motions to
// proceed, motions to recommit, amendments, nominations — are left out: they
// are votes about a bill's path rather than about enacting it, and a model
// that read the statute text and answered "would you pass this" has no opinion
// on them.
func (c *Client) FinalPassageVotes(ctx context.Context, since time.Time) ([]Vote, error) {
	since = since.UTC().Truncate(24 * time.Hour)
	now := c.now().UTC()
	if since.After(now) {
		return nil, nil
	}

	var out []Vote
	var firstErr error
	note := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	for year := since.Year(); year <= now.Year(); year++ {
		house, err := c.houseIndex(ctx, year)
		note(err)
		out = append(out, house...)

		senate, err := c.senateIndex(ctx, congressFor(year), sessionFor(year))
		note(err)
		out = append(out, senate...)
	}

	kept := out[:0]
	for _, v := range out {
		if v.BillID == "" || !IsFinalPassage(v.Question) || v.Date.Before(since) {
			continue
		}
		kept = append(kept, v)
	}
	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Date.Before(kept[j].Date) })

	// A reachable chamber's votes are worth returning even when the other
	// chamber's index is down; the error is only fatal if nothing came back.
	if len(kept) == 0 && firstErr != nil {
		return nil, firstErr
	}
	if firstErr != nil {
		c.logf("rollcall: continuing with %d roll call(s) after an index error: %v", len(kept), firstErr)
	}
	return kept, nil
}

// Positions fetches every member's recorded position on one roll call.
func (c *Client) Positions(ctx context.Context, v Vote) ([]Position, error) {
	if v.Chamber == ChamberSenate {
		return c.senatePositions(ctx, v)
	}
	return c.housePositions(ctx, v)
}

// ---------------------------------------------------------------- the House

// houseRow matches one row of the Clerk's roll call index, which is a plain
// HTML table: roll number, date, issue, question, result, description.
var (
	houseRowRE  = regexp.MustCompile(`(?is)<tr>(.*?)</tr>`)
	houseCellRE = regexp.MustCompile(`(?is)<td[^>]*>(.*?)</td>`)
	congressRE  = regexp.MustCompile(`/bill/(\d+)[a-z]{2}-congress/`)
	tagRE       = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRE     = regexp.MustCompile(`\s+`)
)

// houseIndex reads the Clerk's index pages for a year. They are paginated a
// hundred roll calls at a time — ROLL_000, ROLL_100, ... — and the first page
// past the end is a 404, which is how the run ends.
func (c *Client) houseIndex(ctx context.Context, year int) ([]Vote, error) {
	var out []Vote
	for page := 0; page < maxIndexPages*100; page += 100 {
		url := fmt.Sprintf("%s/%d/ROLL_%03d.asp", c.houseBase, year, page)
		body, err := c.fetch(ctx, url)
		if errors.Is(err, errNotFound) {
			break
		}
		if err != nil {
			return out, err
		}
		out = append(out, parseHouseIndex(string(body), year)...)
	}
	c.logf("rollcall: %d House roll call(s) indexed for %d", len(out), year)
	return out, nil
}

func parseHouseIndex(page string, year int) []Vote {
	var out []Vote
	for _, row := range houseRowRE.FindAllStringSubmatch(page, -1) {
		cells := houseCellRE.FindAllStringSubmatch(row[1], -1)
		if len(cells) < 6 {
			continue
		}
		number, err := strconv.Atoi(text(cells[0][1]))
		if err != nil {
			continue // the header row
		}
		congress := congressFor(year)
		if m := congressRE.FindStringSubmatch(cells[2][1]); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				congress = n
			}
		}
		id, label, ok := billIdentity(text(cells[2][1]), congress)
		if !ok {
			continue
		}
		out = append(out, Vote{
			Chamber:   ChamberHouse,
			Congress:  congress,
			Session:   sessionFor(year),
			Year:      year,
			Number:    number,
			Date:      parseDayMonth(text(cells[1][1]), year),
			BillID:    id,
			BillLabel: label,
			Question:  text(cells[3][1]),
			Result:    text(cells[4][1]),
			Title:     text(cells[5][1]),
			URL:       fmt.Sprintf("https://clerk.house.gov/Votes/%d%d", year, number),
		})
	}
	return out
}

type houseRoll struct {
	Metadata struct {
		VoteType string `xml:"vote-type"`
	} `xml:"vote-metadata"`
	Votes []struct {
		Legislator struct {
			NameID    string `xml:"name-id,attr"`
			SortField string `xml:"sort-field,attr"`
			Party     string `xml:"party,attr"`
			State     string `xml:"state,attr"`
		} `xml:"legislator"`
		Vote string `xml:"vote"`
	} `xml:"vote-data>recorded-vote"`
}

func (c *Client) housePositions(ctx context.Context, v Vote) ([]Position, error) {
	url := fmt.Sprintf("%s/%d/roll%03d.xml", c.houseBase, v.Year, v.Number)
	body, err := c.fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	var roll houseRoll
	if err := decodeXML(body, &roll); err != nil {
		return nil, fmt.Errorf("rollcall: decode %s: %w", url, err)
	}
	// A quorum call or a vote taken by a means other than the electronic
	// device is not a division on the question, so it is not comparable.
	if kind := strings.ToUpper(roll.Metadata.VoteType); !strings.Contains(kind, "YEA-AND-NAY") && !strings.Contains(kind, "RECORDED VOTE") {
		return nil, fmt.Errorf("rollcall: House roll %d of %d is a %q, not a division", v.Number, v.Year, roll.Metadata.VoteType)
	}

	out := make([]Position, 0, len(roll.Votes))
	for _, rv := range roll.Votes {
		out = append(out, Position{
			MemberID: rv.Legislator.NameID,
			Surname:  rv.Legislator.SortField,
			State:    rv.Legislator.State,
			Party:    rv.Legislator.Party,
			Cast:     NormalizeCast(rv.Vote),
		})
	}
	return out, nil
}

// --------------------------------------------------------------- the Senate

type senateMenu struct {
	Congress int `xml:"congress"`
	Session  int `xml:"session"`
	Year     int `xml:"congress_year"`
	Votes    []struct {
		Number   string `xml:"vote_number"`
		Date     string `xml:"vote_date"`
		Issue    string `xml:"issue"`
		Question string `xml:"question"`
		Result   string `xml:"result"`
		Title    string `xml:"title"`
	} `xml:"votes>vote"`
}

func (c *Client) senateIndex(ctx context.Context, congress, session int) ([]Vote, error) {
	url := fmt.Sprintf("%s/roll_call_lists/vote_menu_%d_%d.xml", c.senateBase, congress, session)
	body, err := c.fetch(ctx, url)
	// A session the Senate has not opened yet simply has no menu.
	if errors.Is(err, errNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var menu senateMenu
	if err := decodeXML(body, &menu); err != nil {
		return nil, fmt.Errorf("rollcall: decode %s: %w", url, err)
	}
	year := menu.Year
	if year == 0 {
		year = yearFor(congress, session)
	}

	out := make([]Vote, 0, len(menu.Votes))
	for _, v := range menu.Votes {
		number, err := strconv.Atoi(strings.TrimLeft(strings.TrimSpace(v.Number), "0"))
		if err != nil {
			continue
		}
		id, label, ok := billIdentity(v.Issue, congress)
		if !ok {
			continue
		}
		out = append(out, Vote{
			Chamber:   ChamberSenate,
			Congress:  congress,
			Session:   session,
			Year:      year,
			Number:    number,
			Date:      parseDayMonth(v.Date, year),
			BillID:    id,
			BillLabel: label,
			Question:  text(v.Question),
			Result:    text(v.Result),
			Title:     text(v.Title),
			URL: fmt.Sprintf("https://www.senate.gov/legislative/LIS/roll_call_votes/vote%d%d/vote_%d_%d_%05d.htm",
				congress, session, congress, session, number),
		})
	}
	c.logf("rollcall: %d Senate roll call(s) indexed for congress %d session %d", len(out), congress, session)
	return out, nil
}

type senateRoll struct {
	Members []struct {
		LastName string `xml:"last_name"`
		Party    string `xml:"party"`
		State    string `xml:"state"`
		Cast     string `xml:"vote_cast"`
		LISID    string `xml:"lis_member_id"`
	} `xml:"members>member"`
}

func (c *Client) senatePositions(ctx context.Context, v Vote) ([]Position, error) {
	url := fmt.Sprintf("%s/roll_call_votes/vote%d%d/vote_%d_%d_%05d.xml",
		c.senateBase, v.Congress, v.Session, v.Congress, v.Session, v.Number)
	body, err := c.fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	var roll senateRoll
	if err := decodeXML(body, &roll); err != nil {
		return nil, fmt.Errorf("rollcall: decode %s: %w", url, err)
	}
	out := make([]Position, 0, len(roll.Members))
	for _, m := range roll.Members {
		out = append(out, Position{
			MemberID: strings.TrimSpace(m.LISID),
			Surname:  strings.TrimSpace(m.LastName),
			State:    strings.TrimSpace(m.State),
			Party:    strings.TrimSpace(m.Party),
			Cast:     NormalizeCast(m.Cast),
		})
	}
	return out, nil
}

// ------------------------------------------------------------------ parsing

// finalPassage are the questions that put the bill itself to the chamber. A
// question has to say it is passing, concurring in the other chamber's text,
// or overriding a veto; everything else is procedure.
var finalPassage = []string{
	"on passage",
	"suspend the rules and pass",
	"suspend the rules and concur",
	"concur in the senate amendment",
	"concurring in senate amendment",
	"concur in the house amendment",
	"concurring in house amendment",
	"overriding the veto",
}

// IsFinalPassage reports whether a roll call question is a vote on enacting
// the bill rather than on how it is handled. Cloture, motions to proceed,
// motions to recommit, amendments and nominations are all false.
func IsFinalPassage(question string) bool {
	q := strings.ToLower(text(question))
	for _, marker := range finalPassage {
		if strings.Contains(q, marker) {
			return true
		}
	}
	return false
}

// NormalizeCast maps the words the two chambers print onto the four positions.
// The House says Aye and No in the Committee of the Whole and Yea and Nay on
// the floor; the Senate always says Yea and Nay.
func NormalizeCast(raw string) string {
	switch strings.ToLower(text(raw)) {
	case "yea", "aye", "yes":
		return Yea
	case "nay", "no":
		return Nay
	case "present", "present - announced":
		return Present
	case "not voting", "absent":
		return NotVoting
	}
	return ""
}

var billNumberRE = regexp.MustCompile(`^\d+$`)

// billIdentity turns a roll call's "issue" column into the corpus bill id.
// Both chambers write bills a little differently — "H R 8884" from the Clerk,
// "S. 5271" from the Senate — and everything that is not a House or Senate
// bill, including simple and concurrent resolutions, amendments and
// nominations, is rejected: this project only tracks HR and S.
func billIdentity(issue string, congress int) (id, label string, ok bool) {
	fields := strings.Fields(strings.ToUpper(strings.ReplaceAll(text(issue), ".", " ")))
	if len(fields) < 2 {
		return "", "", false
	}
	var kind, number string
	switch {
	case fields[0] == "H" && fields[1] == "R" && len(fields) > 2 && billNumberRE.MatchString(fields[2]):
		kind, number = "hr", fields[2]
	case fields[0] == "HR" && billNumberRE.MatchString(fields[1]):
		kind, number = "hr", fields[1]
	case fields[0] == "S" && billNumberRE.MatchString(fields[1]):
		kind, number = "s", fields[1]
	default:
		return "", "", false
	}
	label = "H.R. " + number
	if kind == "s" {
		label = "S. " + number
	}
	return fmt.Sprintf("%s-%s-%d", kind, number, congress), label, true
}

// parseDayMonth reads the "23-Jul" dates both indexes print, which carry the
// year only in the page around them.
func parseDayMonth(value string, year int) time.Time {
	value = text(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{"2-Jan-2006", "January 2, 2006", "2006-01-02"} {
		candidate := value
		if layout == "2-Jan-2006" {
			candidate = fmt.Sprintf("%s-%d", value, year)
		}
		if t, err := time.Parse(layout, candidate); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// congressFor returns the congress sitting in a calendar year. Each congress
// runs for two years and begins in an odd one.
func congressFor(year int) int { return (year-1789)/2 + 1 }

// sessionFor returns the session of the congress a year falls in: the first
// runs through the odd year, the second through the even one.
func sessionFor(year int) int {
	if year%2 == 1 {
		return 1
	}
	return 2
}

func yearFor(congress, session int) int { return 1789 + (congress-1)*2 + (session - 1) }

// text strips any markup and collapses the whitespace both feeds indent with.
func text(s string) string {
	s = tagRE.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = strings.NewReplacer("&nbsp;", " ", "&amp;", "&", "&quot;", `"`, "&#39;", "'").Replace(s)
	return strings.TrimSpace(spaceRE.ReplaceAllString(s, " "))
}

// decodeXML is lenient on purpose: these documents are decades-old publishing
// pipelines that emit stray entities and a DTD reference that cannot be
// resolved, none of which should cost us a roll call.
func decodeXML(body []byte, out any) error {
	dec := xml.NewDecoder(strings.NewReader(string(body)))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	return dec.Decode(out)
}
