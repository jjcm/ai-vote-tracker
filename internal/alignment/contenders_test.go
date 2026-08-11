package alignment

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/contenders"
	"github.com/pwnies/ai-vote-tracker/internal/models"
)

// Real members of the watch list, so the fixtures exercise the same lookup the
// service does. Ocasio-Cortez and Massie sit in the House, Booker in the
// Senate.
const (
	aoc    = "O000172"
	massie = "M001184"
	booker = "B001288"
)

func votedBill(id string, day int, votes map[string]string) models.BillWithVotes {
	b := models.BillWithVotes{Bill: models.Bill{
		ID:        id,
		Number:    strings.ToUpper(id),
		UpdatedAt: time.Date(2026, 6, day, 0, 0, 0, 0, time.UTC),
	}}
	for _, m := range models.Catalog {
		v := models.Vote{BillID: id, ModelKey: m.Key, ModelName: m.Name, Vote: models.VotePending}
		if cast, ok := votes[m.Key]; ok {
			v.Vote = cast
		}
		b.Votes = append(b.Votes, v)
	}
	return b
}

func floor(billID, bioguide, position string, day int) models.MemberVote {
	return models.MemberVote{
		BillID:   billID,
		Bioguide: bioguide,
		Position: position,
		VotedAt:  time.Date(2026, 6, day, 0, 0, 0, 0, time.UTC),
	}
}

func matchFor(t *testing.T, res ContenderResult, key string) ModelContenders {
	t.Helper()
	for _, m := range res.Models {
		if m.Key == key {
			return m
		}
	}
	t.Fatalf("no contender ranking computed for %q", key)
	return ModelContenders{}
}

// A model that voted with a member every time is a perfect match, and the
// pairing survives to the top of the ranking.
func TestAgreementCountsMatchingSides(t *testing.T) {
	bills := []models.BillWithVotes{
		votedBill("hr-1-119", 1, map[string]string{"opus": models.VoteYes}),
		votedBill("hr-2-119", 2, map[string]string{"opus": models.VoteNo}),
		votedBill("hr-3-119", 3, map[string]string{"opus": models.VoteYes}),
	}
	positions := []models.MemberVote{
		floor("hr-1-119", aoc, models.PositionYea, 1),
		floor("hr-2-119", aoc, models.PositionNay, 2),
		floor("hr-3-119", aoc, models.PositionYea, 3),
	}

	opus := matchFor(t, MatchContenders(bills, positions, 3), "opus")
	if opus.Closest == nil {
		t.Fatal("three shared votes should be enough to rank a member")
	}
	if opus.Closest.Bioguide != aoc {
		t.Errorf("closest = %q, want Ocasio-Cortez", opus.Closest.Name)
	}
	if opus.Closest.Agreement != 1 || opus.Closest.Agreed != 3 || opus.Closest.Overlap != 3 {
		t.Errorf("agreement = %v (%d of %d), want 3 of 3", opus.Closest.Agreement, opus.Closest.Agreed, opus.Closest.Overlap)
	}
	if len(opus.Closest.Divergences) != 0 {
		t.Errorf("a perfect match cannot have split votes: %v", opus.Closest.Divergences)
	}
	if !strings.Contains(opus.Explanation, "have not split once") {
		t.Errorf("explanation = %q", opus.Explanation)
	}
}

// A disagreement lowers the rate and is named, so the page can say where a
// close match came apart.
func TestAgreementRateAndDivergences(t *testing.T) {
	bills := []models.BillWithVotes{
		votedBill("hr-1-119", 1, map[string]string{"opus": models.VoteYes}),
		votedBill("hr-2-119", 2, map[string]string{"opus": models.VoteYes}),
		votedBill("hr-3-119", 3, map[string]string{"opus": models.VoteYes}),
		votedBill("hr-4-119", 4, map[string]string{"opus": models.VoteYes}),
	}
	positions := []models.MemberVote{
		floor("hr-1-119", aoc, models.PositionYea, 1),
		floor("hr-2-119", aoc, models.PositionYea, 2),
		floor("hr-3-119", aoc, models.PositionYea, 3),
		floor("hr-4-119", aoc, models.PositionNay, 4),
	}

	closest := matchFor(t, MatchContenders(bills, positions, 3), "opus").Closest
	if closest.Agreement != 0.75 || closest.Agreed != 3 || closest.Overlap != 4 {
		t.Fatalf("agreement = %v (%d of %d), want 0.75 (3 of 4)", closest.Agreement, closest.Agreed, closest.Overlap)
	}
	if len(closest.Divergences) != 1 || closest.Divergences[0] != "HR-4-119" {
		t.Errorf("divergences = %v, want the one bill they split on", closest.Divergences)
	}
}

// Present, Not Voting and a verdict the model never reached leave the pair
// with nothing to compare, so they drop out of the denominator rather than
// counting as a disagreement.
func TestUndecidedPositionsAreNotComparisons(t *testing.T) {
	bills := []models.BillWithVotes{
		votedBill("hr-1-119", 1, map[string]string{"opus": models.VoteYes}),
		votedBill("hr-2-119", 2, map[string]string{"opus": models.VoteYes}),
		votedBill("hr-3-119", 3, map[string]string{"opus": models.VoteError}),
		votedBill("hr-4-119", 4, nil), // still pending
	}
	positions := []models.MemberVote{
		floor("hr-1-119", aoc, models.PositionYea, 1),
		floor("hr-2-119", aoc, models.PositionPresent, 2),
		floor("hr-3-119", aoc, models.PositionYea, 3),
		floor("hr-4-119", aoc, models.PositionNay, 4),
	}

	res := MatchContenders(bills, positions, 1)
	closest := matchFor(t, res, "opus").Closest
	if closest.Overlap != 1 || closest.Agreed != 1 {
		t.Errorf("overlap = %d agreed = %d, want the single bill both took a side on", closest.Overlap, closest.Agreed)
	}
	if res.BillsWithFloorVotes != 4 {
		t.Errorf("all four bills carry a floor vote, got %d", res.BillsWithFloorVotes)
	}
}

// Below the overlap floor a pair is reported but never ranked: one shared vote
// makes anybody a hundred per cent match.
func TestThinOverlapIsNotRanked(t *testing.T) {
	bills := []models.BillWithVotes{votedBill("hr-1-119", 1, map[string]string{"opus": models.VoteYes})}
	positions := []models.MemberVote{floor("hr-1-119", booker, models.PositionYea, 1)}

	opus := matchFor(t, MatchContenders(bills, positions, 3), "opus")
	if opus.Closest != nil {
		t.Fatalf("one shared vote must not produce a closest match, got %s", opus.Closest.Name)
	}
	if !opus.InsufficientOverlap {
		t.Error("the model should be flagged as having too little overlap")
	}
	if len(opus.Unranked) != 1 || opus.Unranked[0].Ranked {
		t.Fatalf("the pair should be reported as unranked, got %+v", opus.Unranked)
	}
	if !strings.Contains(opus.Explanation, "3 floor votes") {
		t.Errorf("explanation should name the floor, got %q", opus.Explanation)
	}
}

// The House votes on final passage many times a session and the Senate only a
// handful, so a perfect record over three votes must not outrank a strong one
// over thirty. Three coin flips landing the same way is not evidence.
func TestRankingDiscountsThinRecords(t *testing.T) {
	var bills []models.BillWithVotes
	var positions []models.MemberVote
	for i := 1; i <= 33; i++ {
		id := fmt.Sprintf("hr-%d-119", i)
		bills = append(bills, votedBill(id, 1, map[string]string{"opus": models.VoteYes}))
		// Massie is on the record for all thirty-three and splits on three of
		// them; Booker's chamber only voted on the first three.
		cast := models.PositionYea
		if i%11 == 0 {
			cast = models.PositionNay
		}
		positions = append(positions, floor(id, massie, cast, 1))
		if i <= 3 {
			positions = append(positions, floor(id, booker, models.PositionYea, 1))
		}
	}

	opus := matchFor(t, MatchContenders(bills, positions, 3), "opus")
	if opus.Closest.Bioguide != massie {
		t.Errorf("closest = %s (%d of %d), want Massie: 30 of 33 is stronger evidence than 3 of 3",
			opus.Closest.Name, opus.Closest.Agreed, opus.Closest.Overlap)
	}
	// The page still shows the plain rate; only the ordering is adjusted.
	if opus.Top[1].Agreement != 1 || opus.Top[1].Confidence >= opus.Top[1].Agreement {
		t.Errorf("Booker should read as 100%% agreement on a discounted score, got %+v", opus.Top[1])
	}
}

// Ranking is by agreement, and a tie is broken by how much evidence the rate
// rests on rather than by map order.
func TestRankingPrefersAgreementThenEvidence(t *testing.T) {
	var bills []models.BillWithVotes
	var positions []models.MemberVote
	for i := 1; i <= 5; i++ {
		id := "hr-" + string(rune('0'+i)) + "-119"
		bills = append(bills, votedBill(id, i, map[string]string{"opus": models.VoteYes}))
		// Massie matches on all five; Booker only on the first three, and
		// Ocasio-Cortez agrees on three of five.
		positions = append(positions, floor(id, massie, models.PositionYea, i))
		if i <= 3 {
			positions = append(positions, floor(id, booker, models.PositionYea, i))
		}
		cast := models.PositionYea
		if i > 3 {
			cast = models.PositionNay
		}
		positions = append(positions, floor(id, aoc, cast, i))
	}

	opus := matchFor(t, MatchContenders(bills, positions, 3), "opus")
	if len(opus.Top) != 3 {
		t.Fatalf("want the top three, got %d", len(opus.Top))
	}
	if opus.Top[0].Bioguide != massie {
		t.Errorf("first = %s, want Massie: same rate as Booker but on more votes", opus.Top[0].Name)
	}
	if opus.Top[1].Bioguide != booker {
		t.Errorf("second = %s, want Booker", opus.Top[1].Name)
	}
	if opus.Top[2].Agreement != 0.6 {
		t.Errorf("third agreement = %v, want 0.6", opus.Top[2].Agreement)
	}
	if opus.ComparableBills != 5 {
		t.Errorf("comparable bills = %d, want 5", opus.ComparableBills)
	}
}

// The later of two floor votes on the same bill is the member's position, and
// a model with no comparable bill at all says so rather than ranking nobody.
func TestModelWithoutFloorVotesExplainsItself(t *testing.T) {
	bills := []models.BillWithVotes{votedBill("hr-1-119", 1, map[string]string{"opus": models.VoteYes})}
	res := MatchContenders(bills, nil, 3)

	opus := matchFor(t, res, "opus")
	if !opus.InsufficientOverlap || opus.Closest != nil {
		t.Fatal("with no floor votes there is nothing to rank")
	}
	if !strings.Contains(opus.Explanation, "No bill in the window") {
		t.Errorf("explanation = %q", opus.Explanation)
	}
	if res.BillsWithFloorVotes != 0 || res.MembersWithVotes != 0 {
		t.Errorf("nothing should be counted: %+v", res)
	}
}

// Positions for somebody who is not on the watch list are ignored: the table
// only ever compares models against 2028 potentials.
func TestVotesFromMembersOffTheListAreIgnored(t *testing.T) {
	bills := []models.BillWithVotes{
		votedBill("hr-1-119", 1, map[string]string{"opus": models.VoteYes}),
		votedBill("hr-2-119", 2, map[string]string{"opus": models.VoteYes}),
		votedBill("hr-3-119", 3, map[string]string{"opus": models.VoteYes}),
	}
	var positions []models.MemberVote
	for i, id := range []string{"hr-1-119", "hr-2-119", "hr-3-119"} {
		positions = append(positions, floor(id, "Z999999", models.PositionYea, i+1))
	}

	res := MatchContenders(bills, positions, 3)
	if res.MembersWithVotes != 0 || res.BillsWithFloorVotes != 0 {
		t.Fatalf("an unknown member should contribute nothing: %+v", res)
	}
	if !matchFor(t, res, "opus").InsufficientOverlap {
		t.Error("no watch list member voted, so no ranking is possible")
	}
}

// The payload names the people who are floated for 2028 but cannot cast a
// floor vote, so the page can say why they carry no score.
func TestPayloadCarriesTheRosterAndTheOutsiders(t *testing.T) {
	res := MatchContenders(nil, nil, 3)
	if len(res.Candidates) != len(contenders.Roster) {
		t.Errorf("candidates = %d, want the whole roster", len(res.Candidates))
	}
	if len(res.NotInCongress) == 0 {
		t.Error("the non-Congress potentials should be listed")
	}
	for _, o := range res.NotInCongress {
		for _, c := range res.Candidates {
			if c.Name == o.Name {
				t.Errorf("%s cannot be both a sitting member and an outsider", o.Name)
			}
		}
	}
}

// The cache signature has to move when any input to the table does.
func TestSignatureTracksItsInputs(t *testing.T) {
	base := Signature(10, 100, 20, 200, 3)
	for name, got := range map[string]string{
		"more model verdicts": Signature(11, 100, 20, 200, 3),
		"a newer verdict":     Signature(10, 101, 20, 200, 3),
		"more floor votes":    Signature(10, 100, 21, 200, 3),
		"a newer floor vote":  Signature(10, 100, 20, 201, 3),
		"a different floor":   Signature(10, 100, 20, 200, 4),
	} {
		if got == base {
			t.Errorf("%s should change the signature", name)
		}
	}
	if Signature(10, 100, 20, 200, 3) != base {
		t.Error("the same inputs must produce the same signature")
	}
	// A table written by an older build is stale however fresh its inputs are.
	if !strings.HasPrefix(base, fmt.Sprint(tableVersion)+":") {
		t.Errorf("signature %q should lead with the table version", base)
	}
}
