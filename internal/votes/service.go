// Package votes orchestrates data bootstrapping and the parallel OpenRouter
// calls that produce each model's verdict.
package votes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/pwnies/ai-vote-tracker/internal/congress"
	"github.com/pwnies/ai-vote-tracker/internal/models"
	"github.com/pwnies/ai-vote-tracker/internal/openrouter"
	"github.com/pwnies/ai-vote-tracker/internal/seed"
	"github.com/pwnies/ai-vote-tracker/internal/store"
)

// Service runs voting rounds and keeps the bill corpus fresh.
type Service struct {
	store    *store.Store
	router   *openrouter.Client
	congress *congress.Client
	logger   *log.Logger

	billLimit int

	mu       sync.Mutex
	inFlight map[string]bool
	source   string
}

// New builds the service. congressClient may be disabled, in which case the
// seed corpus is used.
func New(st *store.Store, router *openrouter.Client, cg *congress.Client, billLimit int, logger *log.Logger) *Service {
	if billLimit <= 0 {
		billLimit = 12
	}
	return &Service{
		store:     st,
		router:    router,
		congress:  cg,
		logger:    logger,
		billLimit: billLimit,
		inFlight:  map[string]bool{},
		source:    models.SourceSeed,
	}
}

// Status describes the current data pipeline state for the UI.
type Status struct {
	Source          string `json:"source"`
	VotingEnabled   bool   `json:"votingEnabled"`
	CongressEnabled bool   `json:"congressEnabled"`
	BillsInFlight   int    `json:"billsInFlight"`
}

// Status returns the current pipeline state.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return Status{
		Source:          s.source,
		VotingEnabled:   s.router.Enabled(),
		CongressEnabled: s.congress.Enabled(),
		BillsInFlight:   len(s.inFlight),
	}
}

// Bootstrap loads bills if the database is empty, then votes the newest bill
// synchronously (so the homepage is never blank) and the rest in the
// background.
func (s *Service) Bootstrap(ctx context.Context, syncTimeout time.Duration) error {
	count, err := s.store.CountBills(ctx)
	if err != nil {
		return err
	}
	if count == 0 {
		if err := s.loadBills(ctx); err != nil {
			return err
		}
	} else {
		s.setSourceFromStore(ctx)
	}

	if purged, err := s.store.PurgeMalformedVotes(ctx); err != nil {
		s.logger.Printf("purging malformed verdicts: %v", err)
	} else if purged > 0 {
		s.logger.Printf("dropped %d verdict(s) with a malformed rationale; they will be re-collected", purged)
	}

	if !s.router.Enabled() {
		s.logger.Printf("OPENROUTER_KEY is not set: bills are loaded but no model votes will be collected")
		return nil
	}

	pending, err := s.store.BillIDsMissingVotes(ctx)
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return nil
	}

	// Vote the featured bill up front so the homepage has content, but never
	// block startup indefinitely on a slow provider.
	featured := pending[0]
	syncCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	if err := s.VoteBill(syncCtx, featured, false); err != nil {
		s.logger.Printf("bootstrap vote for %s did not complete: %v", featured, err)
	}
	cancel()

	go s.backfill(context.WithoutCancel(ctx))
	return nil
}

func (s *Service) backfill(ctx context.Context) {
	pending, err := s.store.BillIDsMissingVotes(ctx)
	if err != nil {
		s.logger.Printf("backfill: %v", err)
		return
	}
	for _, id := range pending {
		if ctx.Err() != nil {
			return
		}
		if err := s.VoteBill(ctx, id, false); err != nil && !errors.Is(err, errAlreadyRunning) {
			s.logger.Printf("backfill vote for %s failed: %v", id, err)
		}
	}
	if err := s.scoreIdeologies(ctx); err != nil {
		s.logger.Printf("ideology scoring: %v", err)
	}
	s.logger.Printf("backfill complete")
}

func (s *Service) loadBills(ctx context.Context) error {
	bills := seed.Bills()
	source := models.SourceSeed

	if s.congress.Enabled() {
		live, err := s.congress.RecentBills(ctx, s.billLimit)
		if err != nil {
			s.logger.Printf("congress.gov fetch failed, falling back to the seed corpus: %v", err)
		} else {
			bills = live
			source = models.SourceCongress
			s.logger.Printf("loaded %d bills with published statute text from congress.gov", len(bills))
		}
	}

	for _, b := range bills {
		if !b.HasStatuteText() {
			s.logger.Printf("skipping %s: it carries no statute text for the models to read", b.ID)
			continue
		}
		changed, err := s.store.UpsertBill(ctx, b)
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		// The stored verdicts were cast on text this bill no longer has —
		// either a superseded print, or the CRS summary an older build fed the
		// models. Either way they are not verdicts on the law now in front of
		// the models, so they are collected again.
		if err := s.invalidate(ctx, b); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.source = source
	s.mu.Unlock()
	if source == models.SourceSeed {
		s.logger.Printf("loaded %d seed bills (set CONGRESS_API_KEY for live Congress.gov data)", len(bills))
	}
	return nil
}

// invalidate drops every model's reading of a bill whose statute text changed.
func (s *Service) invalidate(ctx context.Context, bill models.Bill) error {
	if err := s.store.DeleteDeliberations(ctx, bill.ID); err != nil {
		return err
	}
	if err := s.store.DeleteVotes(ctx, bill.ID); err != nil {
		return err
	}
	s.logger.Printf("statute text for %s changed (%s, %s): dropped the stored memos and verdicts so they are collected against the new text",
		bill.ID, bill.TextVersion, bill.TextSource)
	return nil
}

func (s *Service) setSourceFromStore(ctx context.Context) {
	bills, err := s.store.Bills(ctx)
	if err != nil || len(bills) == 0 {
		return
	}
	s.mu.Lock()
	s.source = bills[0].Source
	s.mu.Unlock()
}

// Refresh re-reads the upstream bill list and votes anything new. It is safe to
// call while the server is running.
func (s *Service) Refresh(ctx context.Context) error {
	if err := s.loadBills(ctx); err != nil {
		return err
	}
	go s.backfill(context.WithoutCancel(ctx))
	return nil
}

var errAlreadyRunning = errors.New("a voting round is already in progress for this bill")

// VoteBill collects verdicts for one bill. When force is true every model is
// re-queried; otherwise only models without a recorded verdict are called.
func (s *Service) VoteBill(ctx context.Context, billID string, force bool) error {
	if !s.router.Enabled() {
		return openrouter.ErrNoKey
	}

	s.mu.Lock()
	if s.inFlight[billID] {
		s.mu.Unlock()
		return errAlreadyRunning
	}
	s.inFlight[billID] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.inFlight, billID)
		s.mu.Unlock()
	}()

	bill, err := s.store.Bill(ctx, billID)
	if err != nil {
		return err
	}
	if !bill.HasStatuteText() {
		return fmt.Errorf("%s has no statute text, so there is nothing for the models to read", bill.ID)
	}

	targets := models.Catalog
	if !force {
		missing, err := s.store.MissingVoteModels(ctx, billID)
		if err != nil {
			return err
		}
		targets = missing
	}
	if len(targets) == 0 {
		return nil
	}

	// One in-flight pipeline per model: five concurrent chains is well within
	// OpenRouter's limits and keeps a full round to roughly one model's latency
	// (plus its own section digests, when the bill needs them).
	var writeMu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(len(models.Catalog))
	for _, m := range targets {
		m := m
		g.Go(func() error {
			record := models.Vote{
				BillID:    bill.ID,
				ModelKey:  m.Key,
				ModelName: m.Name,
				CreatedAt: time.Now().UTC(),
			}
			verdict, err := s.decide(gctx, m, bill, &writeMu)
			if err != nil {
				s.logger.Printf("vote %s/%s failed: %v", bill.ID, m.Key, err)
				record.Vote = models.VoteError
				record.Error = err.Error()
			} else {
				record.Vote = verdict.Vote
				record.Reason = verdict.Reason
			}
			writeMu.Lock()
			defer writeMu.Unlock()
			// Persist with the parent context: a sibling failure cancels gctx,
			// and a completed verdict should not be thrown away because of it.
			return s.store.SaveVote(context.WithoutCancel(ctx), record)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	if bill.IdeologyScore == 0 {
		if score, err := s.router.ScoreIdeology(ctx, bill); err != nil {
			s.logger.Printf("ideology score for %s failed: %v", bill.ID, err)
		} else if score != 0 {
			if err := s.store.SetIdeologyScore(context.WithoutCancel(ctx), bill.ID, score); err != nil {
				s.logger.Printf("store ideology score for %s: %v", bill.ID, err)
			}
		}
	}
	return nil
}

// decide runs one model's full pipeline over a bill: its own pros and cons
// memo — written from the statute text, or from its own section notes when the
// text overruns its context window — and then its vote on that memo.
//
// The memo is cached against a fingerprint of the text it was written from, so
// a re-vote reuses the model's own reasoning instead of improvising a new one.
func (s *Service) decide(ctx context.Context, m models.Model, bill models.Bill, writeMu *sync.Mutex) (openrouter.Verdict, error) {
	memo, err := s.memo(ctx, m, bill, writeMu)
	if err != nil {
		return openrouter.Verdict{}, err
	}
	verdict, err := s.router.Vote(ctx, m, bill, memo)
	if errors.Is(err, openrouter.ErrStaleMemo) {
		// The bill no longer fits this model's budget, and the cached memo has
		// no section notes to vote from. Have the model read it again.
		s.logger.Printf("memo for %s/%s predates the current context budget: re-reading the bill", bill.ID, m.Key)
		if memo, err = s.freshMemo(ctx, m, bill, writeMu); err != nil {
			return openrouter.Verdict{}, err
		}
		verdict, err = s.router.Vote(ctx, m, bill, memo)
	}
	return verdict, err
}

// memo returns the model's own pros and cons memo for a bill, reusing the
// stored one when it was written against the same text.
func (s *Service) memo(ctx context.Context, m models.Model, bill models.Bill, writeMu *sync.Mutex) (models.Deliberation, error) {
	stored, err := s.store.Deliberation(ctx, bill.ID, m.Key)
	switch {
	case err == nil && stored.Usable() && stored.TextHash == bill.TextFingerprint():
		return stored, nil
	case err != nil && !errors.Is(err, store.ErrNotFound):
		return models.Deliberation{}, err
	}
	return s.freshMemo(ctx, m, bill, writeMu)
}

func (s *Service) freshMemo(ctx context.Context, m models.Model, bill models.Bill, writeMu *sync.Mutex) (models.Deliberation, error) {
	memo, err := s.router.Deliberate(ctx, m, bill)
	if err != nil {
		return models.Deliberation{}, err
	}
	writeMu.Lock()
	defer writeMu.Unlock()
	if err := s.store.SaveDeliberation(context.WithoutCancel(ctx), memo); err != nil {
		return models.Deliberation{}, err
	}
	return memo, nil
}

func (s *Service) scoreIdeologies(ctx context.Context) error {
	if !s.router.Enabled() {
		return nil
	}
	bills, err := s.store.BillsNeedingIdeology(ctx)
	if err != nil {
		return err
	}
	for _, b := range bills {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		score, err := s.router.ScoreIdeology(ctx, b)
		if err != nil {
			s.logger.Printf("ideology score for %s failed: %v", b.ID, err)
			continue
		}
		if score == 0 {
			// Zero is the "unscored" sentinel; nudge a truly neutral bill off it.
			score = 0.01
		}
		if err := s.store.SetIdeologyScore(ctx, b.ID, score); err != nil {
			return err
		}
	}
	return nil
}

// IsRunning reports whether a voting round is in progress for a bill.
func (s *Service) IsRunning(billID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inFlight[billID]
}
