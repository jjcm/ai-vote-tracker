// Package congress fetches recent legislation from the Congress.gov API.
//
// The /summaries feed is the useful entry point here: it is ordered by update
// time and already carries the CRS summary text, which is what the models read.
package congress

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// Client is a read-only Congress.gov API client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

// New builds a client. baseURL should include the /v3 suffix.
func New(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Enabled reports whether an API key is configured.
func (c *Client) Enabled() bool { return c != nil && c.apiKey != "" }

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
		Title string `json:"title"`
	} `json:"bill"`
}

// RecentBills returns up to limit recent bills that carry a summary, newest
// first. Resolutions without legislative effect are skipped.
func (c *Client) RecentBills(ctx context.Context, limit int) ([]models.Bill, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("congress: CONGRESS_API_KEY is not set")
	}
	if limit <= 0 {
		limit = 12
	}

	// Over-fetch: the feed contains repeat summaries for the same bill and a
	// fair number of ceremonial resolutions.
	var feed summariesResponse
	if err := c.get(ctx, "/summaries", url.Values{
		"sort":  {"updateDate desc"},
		"limit": {"250"},
	}, &feed); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var out []models.Bill
	for _, s := range feed.Summaries {
		if len(out) >= limit {
			break
		}
		b := s.Bill
		billType := strings.ToUpper(b.Type)
		if billType != "HR" && billType != "S" {
			continue
		}
		id := fmt.Sprintf("%s-%s-%d", strings.ToLower(billType), b.Number, b.Congress)
		if seen[id] {
			continue
		}
		summary := stripHTML(s.Text)
		if len(summary) < 120 {
			continue
		}
		seen[id] = true

		chamber := models.ChamberHouse
		display := "H.R. " + b.Number
		if billType == "S" {
			chamber = models.ChamberSenate
			display = "S. " + b.Number
		}

		bill := models.Bill{
			ID:        id,
			Number:    display,
			Title:     strings.TrimSpace(b.Title),
			Chamber:   chamber,
			Summary:   shorten(summary, 460),
			FullText:  summary,
			Source:    models.SourceCongress,
			SourceURL: legislationURL(b.Congress, billType, b.Number),
			UpdatedAt: parseTime(s.UpdateDate),
			Status:    s.ActionDesc,
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

	if len(out) == 0 {
		return nil, fmt.Errorf("congress: no usable bills in the summaries feed")
	}
	return out, nil
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
