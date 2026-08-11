// Package models defines the core domain types shared across the application:
// the AI model catalog, bills, and the votes those models cast.
package models

import (
	"strings"
	"time"
)

// Model is one of the language models whose vote we track.
type Model struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	OpenRouterID string `json:"openRouterId"`
}

// Catalog is the fixed, ordered list of models shown across the site. Display
// order matches the vote columns in the design.
var Catalog = []Model{
	{Key: "opus", Name: "Opus", OpenRouterID: "anthropic/claude-opus-5"},
	{Key: "grok", Name: "Grok", OpenRouterID: "x-ai/grok-4.5"},
	{Key: "gpt-sol", Name: "GPT Sol", OpenRouterID: "openai/gpt-5.6-sol"},
	{Key: "deepseek", Name: "DeepSeek", OpenRouterID: "deepseek/deepseek-v4-pro"},
	{Key: "gemini", Name: "Gemini", OpenRouterID: "google/gemini-3.6-flash"},
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

// Bill is a single piece of legislation under review.
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
	IdeologyScore  float64   `json:"ideologyScore"`
	Source         string    `json:"source"`
	SourceURL      string    `json:"sourceUrl,omitempty"`
	IntroducedDate time.Time `json:"introducedDate"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Vote is one model's verdict on one bill.
type Vote struct {
	BillID    string    `json:"billId"`
	ModelKey  string    `json:"modelKey"`
	ModelName string    `json:"modelName"`
	Vote      string    `json:"vote"`
	Reason    string    `json:"reason"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

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
