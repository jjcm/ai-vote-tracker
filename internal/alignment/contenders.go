package alignment

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/contenders"
	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// DefaultMinOverlap is how many bills a model and a member both have to have
// taken a binary position on before the pair is ranked. Below it an agreement
// rate is arithmetic rather than evidence: one shared vote makes somebody a
// 100% match.
const DefaultMinOverlap = 3

// ContenderMatch is one (model, member) pair: how often the two landed on the
// same side of the bills they both took a position on.
type ContenderMatch struct {
	Bioguide string `json:"bioguide"`
	Name     string `json:"name"`
	Title    string `json:"title"`
	Party    string `json:"party"`
	State    string `json:"state"`
	Chamber  string `json:"chamber"`
	Note     string `json:"note,omitempty"`
	// Agreement is the share of overlapping votes the two agreed on, 0 to 1.
	Agreement float64 `json:"agreement"`
	Agreed    int     `json:"agreed"`
	Overlap   int     `json:"overlap"`
	// Divergences names the bills they split on, newest first, so the page can
	// say where a close match came apart.
	Divergences []string `json:"divergences,omitempty"`
	// Ranked is false when the pair has too little overlap to be ranked.
	Ranked bool `json:"ranked"`
}

// ModelContenders is one model's ranking of the watch list.
type ModelContenders struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	// Closest is the best-matching member, nil when nobody clears the overlap
	// floor.
	Closest *ContenderMatch `json:"closest"`
	// Top is the leading matches, closest first.
	Top []ContenderMatch `json:"top"`
	// Unranked are the members this model shares some votes with but not
	// enough of them, so the page can say who is close to qualifying.
	Unranked []ContenderMatch `json:"unranked,omitempty"`
	// ComparableBills is how many bills this model voted on that any watch
	// list member also voted on.
	ComparableBills     int    `json:"comparableBills"`
	InsufficientOverlap bool   `json:"insufficientOverlap"`
	Explanation         string `json:"explanation"`
}

// ContenderResult is the whole "closest 2028 contenders" payload.
type ContenderResult struct {
	Models []ModelContenders `json:"models"`
	// Candidates is the watch list itself, so the page can show who was in the
	// running whether or not they matched anybody.
	Candidates []contenders.Candidate `json:"candidates"`
	// NotInCongress are the frequently named potentials with no floor votes.
	NotInCongress []contenders.Outsider `json:"notInCongress"`
	MinOverlap    int                   `json:"minOverlap"`
	// Since is the start of the analysis window, formatted as a date.
	Since string `json:"since,omitempty"`
	// BillsWithFloorVotes is how many bills in the corpus carry a recorded
	// position from at least one member of the watch list.
	BillsWithFloorVotes int `json:"billsWithFloorVotes"`
	MemberPositions     int `json:"memberPositions"`
	MembersWithVotes    int `json:"membersWithVotes"`
}

// MatchContenders ranks every watch list member against every model.
//
// A pair is comparable on a bill only when both took a binary position: the
// model returned Yes or No, and the member is recorded Yea or Nay. A member
// who was absent or voted Present, and a model whose verdict is still pending
// or errored, contribute nothing — they are dropped from the denominator
// rather than counted as a disagreement. Below minOverlap shared votes the
// pair is reported but not ranked.
func MatchContenders(bills []models.BillWithVotes, memberVotes []models.MemberVote, minOverlap int) ContenderResult {
	if minOverlap < 1 {
		minOverlap = DefaultMinOverlap
	}

	// Member positions, by bill then member, and the bill each belongs to.
	byBill := map[string]map[string]models.MemberVote{}
	members := map[string]bool{}
	for _, mv := range memberVotes {
		if _, known := contenders.ByBioguide(mv.Bioguide); !known {
			continue
		}
		if byBill[mv.BillID] == nil {
			byBill[mv.BillID] = map[string]models.MemberVote{}
		}
		byBill[mv.BillID][mv.Bioguide] = mv
		members[mv.Bioguide] = true
	}

	result := ContenderResult{
		Candidates:      contenders.Roster,
		NotInCongress:   contenders.NotInCongress,
		MinOverlap:      minOverlap,
		MemberPositions: len(memberVotes),
	}

	type tally struct {
		overlap, agreed int
		split           []string
	}
	// (model, member) -> tally.
	tallies := map[string]map[string]*tally{}
	for _, m := range models.Catalog {
		tallies[m.Key] = map[string]*tally{}
	}
	comparable := map[string]int{}

	// Newest bill first, so the divergence list reads newest first too.
	ordered := append([]models.BillWithVotes(nil), bills...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].UpdatedAt.After(ordered[j].UpdatedAt) })

	for _, bill := range ordered {
		positions := byBill[bill.ID]
		if len(positions) == 0 {
			continue
		}
		result.BillsWithFloorVotes++
		for _, v := range bill.Votes {
			if !v.Decided() {
				continue
			}
			counted := false
			for bioguide, mv := range positions {
				if !mv.Decided() {
					continue
				}
				counted = true
				t := tallies[v.ModelKey][bioguide]
				if t == nil {
					t = &tally{}
					tallies[v.ModelKey][bioguide] = t
				}
				t.overlap++
				if mv.Agrees(v.Vote) {
					t.agreed++
				} else {
					t.split = append(t.split, bill.Number)
				}
			}
			if counted {
				comparable[v.ModelKey]++
			}
		}
	}
	result.MembersWithVotes = len(members)

	for _, m := range models.Catalog {
		mc := ModelContenders{Key: m.Key, Name: m.Name, ComparableBills: comparable[m.Key]}
		var ranked, unranked []ContenderMatch
		for bioguide, t := range tallies[m.Key] {
			candidate, ok := contenders.ByBioguide(bioguide)
			if !ok || t.overlap == 0 {
				continue
			}
			match := ContenderMatch{
				Bioguide:    candidate.Bioguide,
				Name:        candidate.Name,
				Title:       candidate.Title(),
				Party:       candidate.Party,
				State:       candidate.State,
				Chamber:     candidate.Chamber,
				Note:        candidate.Note,
				Agreement:   round3(float64(t.agreed) / float64(t.overlap)),
				Agreed:      t.agreed,
				Overlap:     t.overlap,
				Divergences: t.split,
				Ranked:      t.overlap >= minOverlap,
			}
			if match.Ranked {
				ranked = append(ranked, match)
			} else {
				unranked = append(unranked, match)
			}
		}
		sortMatches(ranked)
		sortMatches(unranked)

		if len(ranked) > 0 {
			closest := ranked[0]
			mc.Closest = &closest
			if len(ranked) > 3 {
				ranked = ranked[:3]
			}
			mc.Top = ranked
		} else {
			mc.InsufficientOverlap = true
		}
		if len(unranked) > 3 {
			unranked = unranked[:3]
		}
		mc.Unranked = unranked
		mc.Explanation = explain(mc, minOverlap)
		result.Models = append(result.Models, mc)
	}
	return result
}

// sortMatches orders by agreement, then by how much evidence the rate rests
// on, then by name so the ordering never wobbles between requests.
func sortMatches(in []ContenderMatch) {
	sort.SliceStable(in, func(i, j int) bool {
		switch {
		case in[i].Agreement != in[j].Agreement:
			return in[i].Agreement > in[j].Agreement
		case in[i].Overlap != in[j].Overlap:
			return in[i].Overlap > in[j].Overlap
		}
		return in[i].Name < in[j].Name
	})
}

// explain writes the sentence under each model's closest match.
func explain(mc ModelContenders, minOverlap int) string {
	if mc.Closest == nil {
		if mc.ComparableBills == 0 {
			return "No bill in the window has both a verdict from this model and a floor vote from anyone on the watch list."
		}
		return fmt.Sprintf("No member shares the %d floor votes with this model that a ranking needs; the closest pair has too little in common to rate.", minOverlap)
	}
	c := *mc.Closest
	line := fmt.Sprintf("Voted the same way as %s %s (%s-%s) on %d of %s.",
		c.Title, c.Name, c.Party, c.State, c.Agreed, plural(c.Overlap, "shared floor vote"))
	switch len(c.Divergences) {
	case 0:
		return line + " They have not split once."
	case 1:
		return line + fmt.Sprintf(" They split only on %s.", c.Divergences[0])
	case 2:
		return line + fmt.Sprintf(" They split on %s and %s.", c.Divergences[0], c.Divergences[1])
	}
	return line + fmt.Sprintf(" They split on %s and %d others.", c.Divergences[0], len(c.Divergences)-1)
}

// FormatSince renders the analysis window start for the methodology note.
func FormatSince(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02")
}

// Signature identifies the inputs an agreement table was computed from, so a
// cached table can be told apart from a stale one.
func Signature(modelVotes int, modelVotesAt int64, memberVotes int, memberVotesAt int64, minOverlap int) string {
	return strings.Join([]string{
		fmt.Sprint(modelVotes), fmt.Sprint(modelVotesAt),
		fmt.Sprint(memberVotes), fmt.Sprint(memberVotesAt),
		fmt.Sprint(minOverlap), fmt.Sprint(len(contenders.Roster)),
	}, ":")
}

func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
