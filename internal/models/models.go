// Package models defines the core domain types shared across the application:
// the AI model catalog, bills, and the votes those models cast.
package models

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

// Model is one of the language models whose vote we track.
type Model struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	OpenRouterID string `json:"openRouterId"`
	// ContextTokens is the model's advertised context window. Statute text
	// larger than a fraction of it has to be digested section by section
	// before the model can deliberate on it.
	ContextTokens int `json:"contextTokens"`
}

// Catalog is the fixed, ordered list of models shown across the site. Display
// order matches the vote columns in the design.
var Catalog = []Model{
	{Key: "opus", Name: "Opus", OpenRouterID: "anthropic/claude-opus-5", ContextTokens: 1_000_000},
	{Key: "grok", Name: "Grok", OpenRouterID: "x-ai/grok-4.5", ContextTokens: 500_000},
	{Key: "gpt-sol", Name: "GPT Sol", OpenRouterID: "openai/gpt-5.6-sol", ContextTokens: 1_050_000},
	{Key: "deepseek", Name: "DeepSeek", OpenRouterID: "deepseek/deepseek-v4-pro", ContextTokens: 1_048_576},
	{Key: "gemini", Name: "Gemini", OpenRouterID: "google/gemini-3.6-flash", ContextTokens: 1_048_576},
}

// ModelByKey looks up a catalog entry.
func ModelByKey(key string) (Model, bool) {
	for _, m := range Catalog {
		if m.Key == key {
			return m, true
		}
	}
	return Model{}, false
}

// ModelKeys returns the catalog keys in display order.
func ModelKeys() []string {
	keys := make([]string, 0, len(Catalog))
	for _, m := range Catalog {
		keys = append(keys, m.Key)
	}
	return keys
}

// Vote values. Verdicts are strictly binary; Pending and Error are transport
// states used while a model call is queued or has failed.
const (
	VoteYes     = "Yes"
	VoteNo      = "No"
	VotePending = "Pending"
	VoteError   = "Error"
)

// Chambers.
const (
	ChamberSenate = "Senate"
	ChamberHouse  = "House"
)

// Bill sources.
const (
	SourceSeed     = "seed"
	SourceCongress = "congress.gov"
)

// Text sources. The models only ever deliberate on statute text, so a bill
// whose text never arrived is not put to a vote. TextSourceSummary describes
// rows written by builds that fed the CRS summary to the models instead.
const (
	TextSourceCongressXML  = "congress-xml"
	TextSourceCongressHTML = "congress-text"
	TextSourceSeedXML      = "seed-xml"
	TextSourceSummary      = "crs-summary"
)

// Bill is a single piece of legislation under review. FullText holds the
// statute text itself — the bill XML published by Congress.gov where it is
// available — which is the only evidence the models are given.
type Bill struct {
	ID             string    `json:"id"`
	Number         string    `json:"number"`
	Title          string    `json:"title"`
	Chamber        string    `json:"chamber"`
	Status         string    `json:"status"`
	StatusCategory string    `json:"statusCategory"`
	PolicyArea     string    `json:"policyArea,omitempty"`
	Sponsor        string    `json:"sponsor,omitempty"`
	SponsorParty   string    `json:"sponsorParty,omitempty"`
	Summary        string    `json:"summary"`
	FullText       string    `json:"fullText,omitempty"`
	TextSource     string    `json:"textSource,omitempty"`
	TextFormat     string    `json:"textFormat,omitempty"`
	TextVersion    string    `json:"textVersion,omitempty"`
	TextURL        string    `json:"textUrl,omitempty"`
	IdeologyScore  float64   `json:"ideologyScore"`
	Source         string    `json:"source"`
	SourceURL      string    `json:"sourceUrl,omitempty"`
	IntroducedDate time.Time `json:"introducedDate"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// HasStatuteText reports whether the bill carries text a model may vote on. A
// CRS summary does not qualify.
func (b Bill) HasStatuteText() bool {
	return strings.TrimSpace(b.FullText) != "" && b.TextSource != "" && b.TextSource != TextSourceSummary
}

// TextFingerprint identifies the statute text a deliberation was based on, so
// that newly published text invalidates the memo written against the old one.
func (b Bill) TextFingerprint() string { return Fingerprint(b.FullText) }

// Fingerprint is a short content hash used to tie cached model work to the
// exact text it read.
func Fingerprint(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// Vote is one model's verdict on one bill. Pros and Cons are copied from the
// model's own deliberation memo when the record is served to the UI.
type Vote struct {
	BillID    string    `json:"billId"`
	ModelKey  string    `json:"modelKey"`
	ModelName string    `json:"modelName"`
	Vote      string    `json:"vote"`
	Reason    string    `json:"reason"`
	Error     string    `json:"error,omitempty"`
	Pros      []string  `json:"pros,omitempty"`
	Cons      []string  `json:"cons,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Deliberation is the pros/cons memo one model wrote for one bill before
// voting on it, plus the section notes it took if the statute text was too
// large to read in one pass. Every field here is private to a single model:
// notes written by one model are never shown to another.
type Deliberation struct {
	BillID    string    `json:"billId"`
	ModelKey  string    `json:"modelKey"`
	ModelName string    `json:"modelName"`
	Pros      []string  `json:"pros"`
	Cons      []string  `json:"cons"`
	Digest    string    `json:"digest,omitempty"`
	Sections  int       `json:"sections"`
	Overflow  bool      `json:"overflow"`
	TextHash  string    `json:"textHash,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Usable reports whether the memo carries enough for a vote to rest on.
func (d Deliberation) Usable() bool { return len(d.Pros) > 0 || len(d.Cons) > 0 }

// BillWithVotes bundles a bill with every model verdict recorded for it, in
// catalog order, with placeholders for models that have not voted yet.
type BillWithVotes struct {
	Bill
	Votes []Vote `json:"votes"`
}

// Decided reports whether the vote is a real Yes/No verdict.
func (v Vote) Decided() bool { return v.Vote == VoteYes || v.Vote == VoteNo }

// NormalizeVote maps loose model output onto the binary vote values.
func NormalizeVote(raw string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Trim(raw, ".\"' "))) {
	case "yes", "yea", "aye", "y", "true", "support", "pass", "for":
		return VoteYes
	case "no", "nay", "n", "false", "oppose", "against", "fail":
		return VoteNo
	}
	return ""
}

// StatusCategory buckets a Congress "latest action" string into the coarse
// stages used by the bills page filter.
func StatusCategory(status string) string {
	s := strings.ToLower(status)
	switch {
	case strings.Contains(s, "became public law"), strings.Contains(s, "signed by president"), strings.Contains(s, "enacted"):
		return "Enacted"
	case strings.Contains(s, "passed senate"), strings.Contains(s, "passed house"), strings.Contains(s, "passed/agreed to"), strings.Contains(s, "agreed to in"):
		return "Passed Chamber"
	case strings.Contains(s, "reported by"), strings.Contains(s, "ordered to be reported"), strings.Contains(s, "markup"):
		return "Reported"
	case strings.Contains(s, "committee"), strings.Contains(s, "referred to"):
		return "In Committee"
	case strings.Contains(s, "introduced"), strings.Contains(s, "read twice"), strings.Contains(s, "read first time"):
		return "Introduced"
	}
	return "In Progress"
}

// StatusCategories is the ordered list offered in the bills page filter.
var StatusCategories = []string{"Introduced", "In Committee", "Reported", "Passed Chamber", "Enacted", "In Progress"}
