package alignment

import (
	"math"
	"testing"

	"github.com/pwnies/ai-vote-tracker/internal/models"
)

func bill(id string, ideology float64, votes map[string]string) models.BillWithVotes {
	b := models.BillWithVotes{Bill: models.Bill{ID: id, IdeologyScore: ideology}}
	for _, m := range models.Catalog {
		v := models.Vote{BillID: id, ModelKey: m.Key, ModelName: m.Name, Vote: models.VotePending}
		if cast, ok := votes[m.Key]; ok {
			v.Vote = cast
		}
		b.Votes = append(b.Votes, v)
	}
	return b
}

func scoreFor(t *testing.T, res Result, key string) ModelAlignment {
	t.Helper()
	for _, m := range res.Models {
		if m.Key == key {
			return m
		}
	}
	t.Fatalf("no alignment computed for %q", key)
	return ModelAlignment{}
}

func TestComputeYesOnConservativeBillsLeansRight(t *testing.T) {
	bills := []models.BillWithVotes{
		bill("a", 0.8, map[string]string{"opus": models.VoteYes, "grok": models.VoteNo}),
		bill("b", 0.6, map[string]string{"opus": models.VoteYes, "grok": models.VoteNo}),
	}
	res := Compute(bills)

	opus := scoreFor(t, res, "opus")
	if opus.Score != 1 {
		t.Errorf("opus voted yes on every conservative bill, want score 1, got %v", opus.Score)
	}
	if opus.Label != "Conservative" {
		t.Errorf("opus label = %q, want Conservative", opus.Label)
	}

	grok := scoreFor(t, res, "grok")
	if grok.Score != -1 {
		t.Errorf("grok voted no on every conservative bill, want score -1, got %v", grok.Score)
	}
}

func TestComputeMixedVotesLandNearCentre(t *testing.T) {
	bills := []models.BillWithVotes{
		bill("a", 0.5, map[string]string{"opus": models.VoteYes}),
		bill("b", -0.5, map[string]string{"opus": models.VoteYes}),
	}
	opus := scoreFor(t, Compute(bills), "opus")
	if math.Abs(opus.Score) > 0.001 {
		t.Errorf("opposing bills should cancel out, got %v", opus.Score)
	}
	if opus.Label != "Centrist" {
		t.Errorf("label = %q, want Centrist", opus.Label)
	}
}

func TestUnscoredBillsCarryNoSignal(t *testing.T) {
	bills := []models.BillWithVotes{bill("a", 0, map[string]string{"opus": models.VoteYes})}
	opus := scoreFor(t, Compute(bills), "opus")
	if opus.Score != 0 {
		t.Errorf("a bill without an ideology score must not move the needle, got %v", opus.Score)
	}
	if opus.BillsVoted != 1 {
		t.Errorf("the vote should still be counted, got %d", opus.BillsVoted)
	}
}

func TestModelWithoutVotesIsFlagged(t *testing.T) {
	res := Compute([]models.BillWithVotes{bill("a", 0.5, nil)})
	m := scoreFor(t, res, "opus")
	if !m.Insufficent {
		t.Error("a model with no verdicts should be flagged as insufficient data")
	}
}

func TestBandBoundaries(t *testing.T) {
	cases := map[float64]string{
		-1.0: "Strongly Liberal",
		-0.6: "Strongly Liberal",
		-0.5: "Lean Liberal",
		-0.1: "Centrist",
		0.0:  "Centrist",
		0.1:  "Lean Conservative",
		0.5:  "Strongly Conservative",
		1.0:  "Strongly Conservative",
	}
	for score, want := range cases {
		if got := BandFor(score).Label; got != want {
			t.Errorf("BandFor(%v) = %q, want %q", score, got, want)
		}
	}
}
