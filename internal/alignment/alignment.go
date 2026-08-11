// Package alignment turns recorded Yes/No votes into a position on the
// -1.0 (liberal) to +1.0 (conservative) axis shown on the alignment page.
package alignment

import (
	"fmt"
	"math"
	"sort"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// Band is one segment of the score guide.
type Band struct {
	Label string  `json:"label"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Tone  string  `json:"tone"`
}

// Bands is the score guide rendered on the alignment page.
var Bands = []Band{
	{Label: "Strongly Liberal", Min: -1.0, Max: -0.5, Tone: "liberal-strong"},
	{Label: "Lean Liberal", Min: -0.5, Max: -0.1, Tone: "liberal"},
	{Label: "Centrist", Min: -0.1, Max: 0.1, Tone: "centrist"},
	{Label: "Lean Conservative", Min: 0.1, Max: 0.5, Tone: "conservative"},
	{Label: "Strongly Conservative", Min: 0.5, Max: 1.0, Tone: "conservative-strong"},
}

// ModelAlignment is one model's computed position.
type ModelAlignment struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Score       float64 `json:"score"`
	Label       string  `json:"label"`
	Tone        string  `json:"tone"`
	Summary     string  `json:"summary"`
	BillsVoted  int     `json:"billsVoted"`
	YesVotes    int     `json:"yesVotes"`
	NoVotes     int     `json:"noVotes"`
	Coverage    float64 `json:"coverage"`
	Insufficent bool    `json:"insufficientData"`
}

// Result is the full alignment payload.
type Result struct {
	Models     []ModelAlignment       `json:"models"`
	Bands      []Band                 `json:"bands"`
	Bills      []models.BillWithVotes `json:"bills"`
	BillsScored int                   `json:"billsScored"`
}

// Compute derives every model's alignment from the supplied bills. A model's
// score is the ideology-weighted average of its votes: voting Yes on a bill
// with score s contributes +s, voting No contributes -s. Bills with no
// ideology score carry no signal and are skipped.
func Compute(bills []models.BillWithVotes) Result {
	type acc struct {
		weighted float64
		weight   float64
		yes, no  int
		voted    int
	}
	accs := map[string]*acc{}
	for _, m := range models.Catalog {
		accs[m.Key] = &acc{}
	}

	scored := 0
	for _, b := range bills {
		ideology := b.IdeologyScore
		if ideology != 0 {
			scored++
		}
		for _, v := range b.Votes {
			a := accs[v.ModelKey]
			if a == nil || !v.Decided() {
				continue
			}
			a.voted++
			direction := 1.0
			if v.Vote == models.VoteNo {
				direction = -1.0
				a.no++
			} else {
				a.yes++
			}
			if ideology == 0 {
				continue
			}
			a.weighted += direction * ideology
			a.weight += math.Abs(ideology)
		}
	}

	out := Result{Bands: Bands, Bills: bills, BillsScored: scored}
	for _, m := range models.Catalog {
		a := accs[m.Key]
		score := 0.0
		if a.weight > 0 {
			score = clamp(a.weighted/a.weight, -1, 1)
		}
		score = round2(score)
		band := BandFor(score)
		ma := ModelAlignment{
			Key:         m.Key,
			Name:        m.Name,
			Score:       score,
			Label:       displayLabel(band.Label),
			Tone:        band.Tone,
			BillsVoted:  a.voted,
			YesVotes:    a.yes,
			NoVotes:     a.no,
			Insufficent: a.voted == 0,
		}
		if len(bills) > 0 {
			ma.Coverage = round2(float64(a.voted) / float64(len(bills)))
		}
		ma.Summary = summarize(m.Name, ma, bills)
		out.Models = append(out.Models, ma)
	}

	sort.SliceStable(out.Models, func(i, j int) bool { return out.Models[i].Score < out.Models[j].Score })
	return out
}

// BandFor returns the score guide band containing a score.
func BandFor(score float64) Band {
	for _, b := range Bands {
		if score >= b.Min && score < b.Max {
			return b
		}
	}
	if score >= 1.0 {
		return Bands[len(Bands)-1]
	}
	return Bands[0]
}

// displayLabel shortens the score guide wording for the model cards, matching
// the design ("Liberal", "Lean Liberal", "Centrist", ...).
func displayLabel(band string) string {
	switch band {
	case "Strongly Liberal":
		return "Liberal"
	case "Strongly Conservative":
		return "Conservative"
	}
	return band
}

// summarize writes the one-line description under each model card from the
// model's own voting record.
func summarize(name string, ma ModelAlignment, bills []models.BillWithVotes) string {
	if ma.Insufficent {
		return "No votes recorded yet, so no alignment can be computed for this model."
	}

	var progYes, progTotal, consYes, consTotal int
	for _, b := range bills {
		if b.IdeologyScore == 0 {
			continue
		}
		for _, v := range b.Votes {
			if v.ModelKey != ma.Key || !v.Decided() {
				continue
			}
			if b.IdeologyScore < 0 {
				progTotal++
				if v.Vote == models.VoteYes {
					progYes++
				}
			} else {
				consTotal++
				if v.Vote == models.VoteYes {
					consYes++
				}
			}
		}
	}

	if progTotal == 0 && consTotal == 0 {
		return fmt.Sprintf("Voted on %s so far; none of those bills carry an ideology score yet.", plural(ma.BillsVoted, "bill"))
	}
	return fmt.Sprintf("Backed %s of %s and %s of %s.",
		count(progYes), plural(progTotal, "progressive-leaning bill"),
		count(consYes), plural(consTotal, "conservative-leaning bill"))
}

func count(n int) string { return fmt.Sprintf("%d", n) }

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func clamp(v, lo, hi float64) float64 {
	return math.Min(hi, math.Max(lo, v))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
