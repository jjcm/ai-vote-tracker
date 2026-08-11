// Package congress fetches recent legislation from the Congress.gov API.
//
// The /summaries feed is the entry point: it is ordered by update time and
// names the bills that have moved recently. The CRS summary it carries is kept
// for the bill cards only — what the models read is the statute text, pulled
// per bill from the /text endpoint and preferring the Formatted XML version of
// the newest text available.
package congress

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// Text download ceiling. The largest bills Congress publishes run to a few
// megabytes of XML; anything past this is not a bill.
const maxTextBytes = 24 << 20

// DefaultUserAgent identifies this client to Congress.gov. It is not optional:
// congress.gov sits behind a CDN that answers a request carrying no user agent
// with a 403, and answers one carrying a default library user agent with a
// Cloudflare 1010 block from some networks. Either way the reply is a block
// page rather than a bill, so every request this package makes is stamped.
const DefaultUserAgent = "AIVoteTracker/1.0 (+https://github.com/pwnies/ai-vote-tracker)"

// Client is a read-only Congress.gov API client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	logger  *log.Logger
	// since is the oldest legislation the corpus covers. Bills that have not
	// moved since then are not fetched at all.
	since time.Time
}

// New builds a client. baseURL should include the /v3 suffix.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http: &http.Client{
			// Bill XML downloads are slower than the JSON calls around them.
			Timeout:   90 * time.Second,
			Transport: identify(DefaultUserAgent, nil),
		},
	}
}

// WithUserAgent replaces the user agent sent with every request, which is where
// an operator puts contact details. A blank agent is ignored rather than sent:
// an unset environment variable must not turn into a request the CDN blocks.
func (c *Client) WithUserAgent(agent string) *Client {
	if agent = strings.TrimSpace(agent); agent != "" {
		c.http.Transport = identify(agent, nil)
	}
	return c
}

// identifier stamps the user agent on every request that leaves the client, so
// that no call site can forget one.
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
	// A RoundTripper may not modify the request it is handed.
	clone := req.Clone(req.Context())
	clone.Header.Set("User-Agent", t.agent)
	return t.base.RoundTrip(clone)
}

// WithLogger records which text version each bill was read from, and which
// bills had to be skipped for want of one.
func (c *Client) WithLogger(logger *log.Logger) *Client {
	c.logger = logger
	return c
}

// WithSince fixes the analysis window: only legislation that has moved on or
// after this date is fetched. A zero time keeps the old behaviour, which is to
// widen the window until enough bills come back.
func (c *Client) WithSince(t time.Time) *Client {
	c.since = t.UTC()
	return c
}

// Since reports the start of the analysis window, zero when none is set.
func (c *Client) Since() time.Time { return c.since }

// Enabled reports whether an API key is configured.
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

func (c *Client) logf(format string, args ...any) {
	if c.logger == nil {
		return
	}
	c.logger.Printf(format, args...)
}

type summariesResponse struct {
	Summaries []struct {
		ActionDate string `json:"actionDate"`
		ActionDesc string `json:"actionDesc"`
		Text       string `json:"text"`
		UpdateDate string `json:"updateDate"`
		Bill       struct {
			Congress      int    `json:"congress"`
			Number        string `json:"number"`
			OriginChamber string `json:"originChamber"`
			Title         string `json:"title"`
			Type          string `json:"type"`
			URL           string `json:"url"`
		} `json:"bill"`
	} `json:"summaries"`
}

type billResponse struct {
	Bill struct {
		IntroducedDate string `json:"introducedDate"`
		LatestAction   struct {
			ActionDate string `json:"actionDate"`
			Text       string `json:"text"`
		} `json:"latestAction"`
		LegislationURL string `json:"legislationUrl"`
		PolicyArea     struct {
			Name string `json:"name"`
		} `json:"policyArea"`
		Sponsors []struct {
			FullName string `json:"fullName"`
			Party    string `json:"party"`
		} `json:"sponsors"`
		Title      string `json:"title"`
		UpdateDate string `json:"updateDate"`
	} `json:"bill"`
}

type textVersionsResponse struct {
	TextVersions []struct {
		Type    string `json:"type"`
		Date    string `json:"date"`
		Formats []struct {
			URL  string `json:"url"`
			Type string `json:"type"`
		} `json:"formats"`
	} `json:"textVersions"`
}

// billText is the statute text of one bill as published by Congress.gov.
type billText struct {
	Text    string
	Format  string
	Version string
	URL     string
	Source  string
}

// Format labels used by the text versions endpoint.
const (
	formatXML  = "Formatted XML"
	formatHTML = "Formatted Text"
)

// statuteText downloads the newest published text of a bill, preferring the
// Formatted XML of the latest version. Congress publishes a bill's text some
// days after introduction, so a bill with no text yet returns an error and is
// skipped rather than voted on from its summary.
func (c *Client) statuteText(ctx context.Context, congress int, billType, number string) (billText, error) {
	var feed textVersionsResponse
	path := fmt.Sprintf("/bill/%d/%s/%s/text", congress, strings.ToLower(billType), number)
	if err := c.get(ctx, path, url.Values{"limit": {"250"}}, &feed); err != nil {
		return billText{}, err
	}

	type version struct {
		label string
		date  time.Time
		urls  map[string]string
	}
	versions := make([]version, 0, len(feed.TextVersions))
	for _, v := range feed.TextVersions {
		urls := map[string]string{}
		for _, f := range v.Formats {
			if f.URL == "" {
				continue
			}
			label := f.Type
			// Some records name the format only in the file extension.
			if label == "" && strings.HasSuffix(strings.ToLower(f.URL), ".xml") {
				label = formatXML
			}
			urls[label] = f.URL
		}
		versions = append(versions, version{label: v.Type, date: parseTime(v.Date), urls: urls})
	}
	// Newest first, and undated versions last: the endpoint returns them
	// oldest first, and a null date means the printing was never stamped.
	sort.SliceStable(versions, func(i, j int) bool { return versions[i].date.After(versions[j].date) })

	pick := func(format string) (version, string, bool) {
		for _, v := range versions {
			if u, ok := v.urls[format]; ok {
				return v, u, true
			}
		}
		return version{}, "", false
	}

	chosen, rawURL, format, source := version{}, "", formatXML, models.TextSourceCongressXML
	if v, u, ok := pick(formatXML); ok {
		chosen, rawURL = v, u
	} else if v, u, ok := pick(formatHTML); ok {
		// No XML for this bill: the HTML print is the best full text there is.
		chosen, rawURL, format, source = v, u, formatHTML, models.TextSourceCongressHTML
		c.logf("congress: %s%s has no XML text version; falling back to %s (%s)", billType, number, formatHTML, v.label)
	} else {
		return billText{}, fmt.Errorf("congress: %s%s has no readable text version yet", billType, number)
	}

	body, err := c.download(ctx, rawURL)
	if err != nil {
		return billText{}, err
	}
	text := strings.TrimSpace(body)
	if format == formatHTML {
		text = stripHTML(text)
	}
	if len(text) < 400 {
		return billText{}, fmt.Errorf("congress: %s%s text at %s is too short to be a bill (%d chars)", billType, number, rawURL, len(text))
	}
	return billText{
		Text:    text,
		Format:  format,
		Version: chosen.label,
		URL:     rawURL,
		Source:  source,
	}, nil
}

// download fetches a text version file. These live on congress.gov rather than
// behind the API, so they need no key.
func (c *Client) download(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("congress: download %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("congress: download %s returned HTTP %d", rawURL, resp.StatusCode)
	}
	// One byte past the ceiling distinguishes a bill that just fits from one
	// that was cut off, which would otherwise be read as a shorter statute
	// than Congress actually printed.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTextBytes+1))
	if err != nil {
		return "", fmt.Errorf("congress: read %s: %w", rawURL, err)
	}
	if len(body) > maxTextBytes {
		return "", fmt.Errorf("congress: text at %s is larger than the %d byte ceiling", rawURL, maxTextBytes)
	}
	return string(body), nil
}

// RecentBills returns up to limit recent bills whose statute text could be
// downloaded, newest first. Resolutions without legislative effect are skipped,
// and so is any bill whose text Congress has not published yet.
func (c *Client) RecentBills(ctx context.Context, limit int) ([]models.Bill, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("congress: CONGRESS_API_KEY is not set")
	}
	if limit <= 0 {
		limit = 12
	}

	// Without fromDateTime the feed only returns the handful of summaries
	// updated in the last few hours. With an analysis window there is one
	// window and it is the window; without one, widen it until enough bills
	// whose text has actually been published come back, because text lags
	// introduction by days and recency alone does not fill a page.
	windows := []time.Time{}
	if !c.since.IsZero() {
		windows = append(windows, c.since)
	} else {
		now := time.Now().UTC()
		for _, back := range []time.Duration{45 * 24 * time.Hour, 180 * 24 * time.Hour, 3 * 365 * 24 * time.Hour} {
			windows = append(windows, now.Add(-back))
		}
	}

	attempted := map[string]bool{}
	var out []models.Bill
	for _, from := range windows {
		var feed summariesResponse
		err := c.get(ctx, "/summaries", url.Values{
			"sort":         {"updateDate desc"},
			"limit":        {"250"},
			"fromDateTime": {from.Format("2006-01-02T15:04:05Z")},
		}, &feed)
		if err != nil {
			return nil, err
		}
		out = append(out, c.readBills(ctx, feed, limit-len(out), attempted)...)
		if len(out) >= limit {
			break
		}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("congress: no bills with published text in the summaries feed")
	}
	return out, nil
}

// BillsByID fetches specific bills by their corpus identifier — the
// "hr-8884-119" shape this package mints — with their statute text, skipping
// any whose text Congress has not published. It is how the corpus picks up the
// bills that actually reached a floor vote, which the summaries feed alone
// does not guarantee.
func (c *Client) BillsByID(ctx context.Context, ids []string) ([]models.Bill, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("congress: CONGRESS_API_KEY is not set")
	}
	seen := map[string]bool{}
	out := make([]models.Bill, 0, len(ids))
	for _, id := range ids {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		congress, billType, number, ok := parseBillID(id)
		if !ok {
			c.logf("congress: %q is not a bill identifier this client understands", id)
			continue
		}
		bill, err := c.billByNumber(ctx, congress, billType, number)
		if err != nil {
			c.logf("congress: skipping %s: %v", id, err)
			continue
		}
		out = append(out, bill)
	}
	return out, nil
}

// billByNumber assembles one bill from its detail record, its newest CRS
// summary and its statute text.
func (c *Client) billByNumber(ctx context.Context, congress int, billType, number string) (models.Bill, error) {
	detail, err := c.billDetail(ctx, congress, billType, number)
	if err != nil {
		return models.Bill{}, err
	}
	title := strings.TrimSpace(detail.Bill.Title)
	if title == "" {
		return models.Bill{}, fmt.Errorf("congress: %s%s has no title, so the record is not readable yet", billType, number)
	}
	if isCommemorative(title) {
		return models.Bill{}, fmt.Errorf("congress: %s%s is a naming or commemorative bill", billType, number)
	}

	text, err := c.statuteText(ctx, congress, billType, number)
	if err != nil {
		return models.Bill{}, err
	}
	c.logf("congress: %s-%s-%d read the %s text as %s (%d chars) from %s",
		strings.ToLower(billType), number, congress, text.Version, text.Format, len(text.Text), text.URL)

	chamber, display := models.ChamberHouse, "H.R. "+number
	if strings.EqualFold(billType, "S") {
		chamber, display = models.ChamberSenate, "S. "+number
	}
	bill := models.Bill{
		ID:             fmt.Sprintf("%s-%s-%d", strings.ToLower(billType), number, congress),
		Number:         display,
		Title:          title,
		Chamber:        chamber,
		Summary:        shorten(c.latestSummary(ctx, congress, billType, number), 460),
		FullText:       text.Text,
		TextSource:     text.Source,
		TextFormat:     text.Format,
		TextVersion:    text.Version,
		TextURL:        text.URL,
		Status:         detail.Bill.LatestAction.Text,
		PolicyArea:     detail.Bill.PolicyArea.Name,
		Source:         models.SourceCongress,
		SourceURL:      legislationURL(congress, billType, number),
		IntroducedDate: parseTime(detail.Bill.IntroducedDate),
		UpdatedAt:      parseTime(detail.Bill.UpdateDate),
	}
	if len(detail.Bill.Sponsors) > 0 {
		bill.Sponsor = detail.Bill.Sponsors[0].FullName
		bill.SponsorParty = detail.Bill.Sponsors[0].Party
	}
	if detail.Bill.LegislationURL != "" {
		bill.SourceURL = detail.Bill.LegislationURL
	}
	if bill.UpdatedAt.IsZero() {
		bill.UpdatedAt = parseTime(detail.Bill.LatestAction.ActionDate)
	}
	if bill.UpdatedAt.IsZero() {
		bill.UpdatedAt = bill.IntroducedDate
	}
	bill.StatusCategory = models.StatusCategory(bill.Status)
	return bill, nil
}

// latestSummary returns the newest CRS summary for a bill, which the cards
// read better with. No model is ever shown it.
func (c *Client) latestSummary(ctx context.Context, congress int, billType, number string) string {
	var feed summariesResponse
	path := fmt.Sprintf("/bill/%d/%s/%s/summaries", congress, strings.ToLower(billType), number)
	if err := c.get(ctx, path, nil, &feed); err != nil || len(feed.Summaries) == 0 {
		return ""
	}
	newest, at := "", time.Time{}
	for _, s := range feed.Summaries {
		when := parseTime(s.UpdateDate)
		if when.IsZero() {
			when = parseTime(s.ActionDate)
		}
		if newest == "" || !when.Before(at) {
			newest, at = stripHTML(s.Text), when
		}
	}
	return newest
}

var billIDRE = regexp.MustCompile(`^(hr|s)-(\d+)-(\d+)$`)

func parseBillID(id string) (congress int, billType, number string, ok bool) {
	m := billIDRE.FindStringSubmatch(strings.ToLower(strings.TrimSpace(id)))
	if m == nil {
		return 0, "", "", false
	}
	congress, err := strconv.Atoi(m[3])
	if err != nil {
		return 0, "", "", false
	}
	return congress, strings.ToUpper(m[1]), m[2], true
}

// readBills turns feed entries into bills, downloading the statute text for
// each. Bills already tried in an earlier pass are not tried again, and bills
// whose text is not published yet are dropped with a note.
func (c *Client) readBills(ctx context.Context, feed summariesResponse, want int, attempted map[string]bool) []models.Bill {
	var out []models.Bill
	for _, s := range feed.Summaries {
		if len(out) >= want || ctx.Err() != nil {
			break
		}
		b := s.Bill
		billType := strings.ToUpper(b.Type)
		if billType != "HR" && billType != "S" {
			continue
		}
		id := fmt.Sprintf("%s-%s-%d", strings.ToLower(billType), b.Number, b.Congress)
		if attempted[id] {
			continue
		}
		// The CRS summary opens with the bill's short title as its own
		// paragraph, which the card prints directly above it. Drop that echo
		// before clipping, so the card's character budget goes to the summary
		// rather than to a repeat of the headline.
		title := strings.TrimSpace(b.Title)
		summary := models.SummaryWithoutTitleEcho(title, stripHTML(s.Text))
		if len(summary) < 120 || isCommemorative(title) {
			continue
		}
		attempted[id] = true

		chamber := models.ChamberHouse
		display := "H.R. " + b.Number
		if billType == "S" {
			chamber = models.ChamberSenate
			display = "S. " + b.Number
		}

		// The models read the law, so a bill whose text Congress has not
		// published is not a candidate at all.
		text, err := c.statuteText(ctx, b.Congress, billType, b.Number)
		if err != nil {
			c.logf("congress: skipping %s: %v", id, err)
			continue
		}
		c.logf("congress: %s read the %s text as %s (%d chars) from %s",
			id, text.Version, text.Format, len(text.Text), text.URL)

		bill := models.Bill{
			ID:          id,
			Number:      display,
			Title:       title,
			Chamber:     chamber,
			Summary:     shorten(summary, 460),
			FullText:    text.Text,
			TextSource:  text.Source,
			TextFormat:  text.Format,
			TextVersion: text.Version,
			TextURL:     text.URL,
			Source:      models.SourceCongress,
			SourceURL:   legislationURL(b.Congress, billType, b.Number),
			UpdatedAt:   parseTime(s.UpdateDate),
			Status:      s.ActionDesc,
		}
		if bill.UpdatedAt.IsZero() {
			bill.UpdatedAt = parseTime(s.ActionDate)
		}
		bill.IntroducedDate = parseTime(s.ActionDate)

		// One extra call per bill fills in the latest action, policy area, and
		// sponsor, which the summaries feed does not carry.
		if detail, err := c.billDetail(ctx, b.Congress, billType, b.Number); err == nil {
			if detail.Bill.LatestAction.Text != "" {
				bill.Status = detail.Bill.LatestAction.Text
			}
			bill.PolicyArea = detail.Bill.PolicyArea.Name
			if d := parseTime(detail.Bill.IntroducedDate); !d.IsZero() {
				bill.IntroducedDate = d
			}
			if len(detail.Bill.Sponsors) > 0 {
				bill.Sponsor = detail.Bill.Sponsors[0].FullName
				bill.SponsorParty = detail.Bill.Sponsors[0].Party
			}
			if detail.Bill.LegislationURL != "" {
				bill.SourceURL = detail.Bill.LegislationURL
			}
		}
		bill.StatusCategory = models.StatusCategory(bill.Status)
		out = append(out, bill)
	}
	return out
}

// isCommemorative filters out post office namings and similar bills, which
// carry no policy content for a model to weigh.
func isCommemorative(title string) bool {
	t := strings.ToLower(title)
	for _, marker := range []string{
		"designate the facility of the united states postal service",
		"to name the department of veterans affairs",
		"congressional gold medal",
		"commemorative coin",
	} {
		if strings.Contains(t, marker) {
			return true
		}
	}
	return false
}

func (c *Client) billDetail(ctx context.Context, congress int, billType, number string) (billResponse, error) {
	var out billResponse
	path := fmt.Sprintf("/bill/%d/%s/%s", congress, strings.ToLower(billType), number)
	err := c.get(ctx, path, nil, &out)
	return out, err
}

func (c *Client) get(ctx context.Context, path string, params url.Values, out any) error {
	if params == nil {
		params = url.Values{}
	}
	params.Set("api_key", c.apiKey)
	params.Set("format", "json")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("congress: GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("congress: read %s: %w", path, err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("congress: GET %s returned HTTP %d", path, resp.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("congress: decode %s: %w", path, err)
	}
	return nil
}

var (
	tagRE   = regexp.MustCompile(`(?s)<[^>]*>`)
	spaceRE = regexp.MustCompile(`[ \t]+`)
	breakRE = regexp.MustCompile(`\n{3,}`)
)

// stripHTML turns the CRS summary markup into readable plain text.
func stripHTML(s string) string {
	s = strings.NewReplacer("</p>", "\n\n", "<br>", "\n", "<br/>", "\n", "<br />", "\n", "</li>", "\n").Replace(s)
	s = tagRE.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = spaceRE.ReplaceAllString(s, " ")
	s = breakRE.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// shorten clips text to a sentence boundary near max characters.
func shorten(s string, max int) string {
	s = strings.Join(strings.Fields(strings.ReplaceAll(s, "\n", " ")), " ")
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	if idx := strings.LastIndex(cut, ". "); idx > max/2 {
		return cut[:idx+1]
	}
	if idx := strings.LastIndex(cut, " "); idx > 0 {
		cut = cut[:idx]
	}
	return strings.TrimRight(cut, " ,;:") + "…"
}

func parseTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func legislationURL(congress int, billType, number string) string {
	slug := "house-bill"
	if strings.EqualFold(billType, "S") {
		slug = "senate-bill"
	}
	return fmt.Sprintf("https://www.congress.gov/bill/%s-congress/%s/%s", ordinal(congress), slug, number)
}

func ordinal(n int) string {
	suffix := "th"
	if n%100 < 11 || n%100 > 13 {
		switch n % 10 {
		case 1:
			suffix = "st"
		case 2:
			suffix = "nd"
		case 3:
			suffix = "rd"
		}
	}
	return fmt.Sprintf("%d%s", n, suffix)
}
