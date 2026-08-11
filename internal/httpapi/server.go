// Package httpapi wires the JSON API and the static site together.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pwnies/ai-vote-tracker/internal/alignment"
	"github.com/pwnies/ai-vote-tracker/internal/models"
	"github.com/pwnies/ai-vote-tracker/internal/store"
	"github.com/pwnies/ai-vote-tracker/internal/votes"
)

// Server serves the API and the static frontend.
type Server struct {
	store  *store.Store
	votes  *votes.Service
	web    fs.FS
	logger *log.Logger
	// minOverlap is how many shared floor votes a model and a member of
	// Congress need before their agreement rate is ranked.
	minOverlap int
}

// New builds the HTTP server. web is the filesystem holding the static site.
func New(st *store.Store, svc *votes.Service, web fs.FS, logger *log.Logger) *Server {
	return &Server{
		store: st, votes: svc, web: web, logger: logger,
		minOverlap: alignment.DefaultMinOverlap,
	}
}

// WithContenderOverlap sets the minimum number of shared floor votes a
// model-to-member comparison has to rest on.
func (s *Server) WithContenderOverlap(n int) *Server {
	if n > 0 {
		s.minOverlap = n
	}
	return s
}

// pageRoutes maps clean URLs onto the static HTML files.
var pageRoutes = map[string]string{
	"/":          "index.html",
	"/bills":     "bills.html",
	"/alignment": "alignment.html",
	"/about":     "about.html",
}

// Handler returns the fully routed handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/bills", s.handleBills)
	mux.HandleFunc("GET /api/bills/{id}", s.handleBill)
	mux.HandleFunc("POST /api/bills/{id}/vote", s.handleVote)
	mux.HandleFunc("GET /api/featured", s.handleFeatured)
	mux.HandleFunc("GET /api/alignment", s.handleAlignment)
	mux.HandleFunc("GET /api/models", s.handleModels)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("POST /api/refresh", s.handleRefresh)

	// /bills/{id} is a client-rendered page: it fetches the bill it was asked
	// for from the API, so every id serves the same document.
	mux.HandleFunc("GET /bills/{id}", func(w http.ResponseWriter, r *http.Request) {
		s.servePage(w, r, "bill.html")
	})

	static := http.FileServer(http.FS(s.web))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if file, ok := pageRoutes[r.URL.Path]; ok {
			s.servePage(w, r, file)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeError(w, http.StatusNotFound, "unknown endpoint")
			return
		}
		if _, err := fs.Stat(s.web, strings.TrimPrefix(path.Clean(r.URL.Path), "/")); err != nil {
			s.serveNotFound(w, r)
			return
		}
		setCache(w, r.URL.Path)
		static.ServeHTTP(w, r)
	})

	return logRequests(s.logger, mux)
}

func (s *Server) servePage(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(s.web, name)
	if err != nil {
		http.Error(w, "page not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, time.Time{}, strings.NewReader(string(data)))
}

func (s *Server) serveNotFound(w http.ResponseWriter, r *http.Request) {
	data, err := fs.ReadFile(s.web, "404.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	w.Write(data)
}

func setCache(w http.ResponseWriter, urlPath string) {
	switch path.Ext(urlPath) {
	case ".webp", ".png", ".svg", ".woff2", ".ico":
		w.Header().Set("Cache-Control", "public, max-age=86400")
	default:
		w.Header().Set("Cache-Control", "no-cache")
	}
}

type billsResponse struct {
	Bills      []models.BillWithVotes `json:"bills"`
	Total      int                    `json:"total"`
	Page       int                    `json:"page"`
	PerPage    int                    `json:"perPage"`
	TotalPages int                    `json:"totalPages"`
	Models     []models.Model         `json:"models"`
	Chambers   []string               `json:"chambers"`
	Statuses   []string               `json:"statuses"`
	Status     votes.Status           `json:"pipeline"`
}

func (s *Server) handleBills(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	all, err := s.loadAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	chamber := strings.TrimSpace(r.URL.Query().Get("chamber"))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	modelKey := strings.TrimSpace(r.URL.Query().Get("model"))
	vote := models.NormalizeVote(r.URL.Query().Get("vote"))

	filtered := make([]models.BillWithVotes, 0, len(all))
	for _, b := range all {
		if q != "" && !matches(b, q) {
			continue
		}
		if chamber != "" && !strings.EqualFold(b.Chamber, chamber) {
			continue
		}
		if status != "" && !strings.EqualFold(b.StatusCategory, status) {
			continue
		}
		if modelKey != "" && vote != "" && !hasVote(b, modelKey, vote) {
			continue
		}
		filtered = append(filtered, b)
	}

	perPage := clampInt(intParam(r, "perPage", 8), 1, 100)
	totalPages := (len(filtered) + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}
	page := clampInt(intParam(r, "page", 1), 1, totalPages)
	start := (page - 1) * perPage
	end := start + perPage
	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	statuses := statusOptions(all)
	writeJSON(w, http.StatusOK, billsResponse{
		Bills:      withoutMemos(filtered[start:end]),
		Total:      len(filtered),
		Page:       page,
		PerPage:    perPage,
		TotalPages: totalPages,
		Models:     models.Catalog,
		Chambers:   []string{models.ChamberSenate, models.ChamberHouse},
		Statuses:   statuses,
		Status:     s.votes.Status(),
	})
}

func (s *Server) handleBill(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	bill, err := s.store.Bill(ctx, r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "bill not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	decorated, err := s.store.WithVotes(ctx, []models.Bill{bill})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	decorated = presentable(decorated)
	// Bill XML runs to megabytes, so the statute text is opt-in rather than the
	// default payload for a page that only needs the verdicts.
	textChars := len(decorated[0].FullText)
	if r.URL.Query().Get("text") != "true" {
		decorated[0].FullText = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bill":      decorated[0],
		"textChars": textChars,
		"voting":    s.votes.IsRunning(bill.ID),
		"models":    models.Catalog,
		"pipeline":  s.votes.Status(),
	})
}

func (s *Server) handleFeatured(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	all, err := s.loadAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(all) == 0 {
		writeError(w, http.StatusNotFound, "no bills loaded yet")
		return
	}
	latest := 8
	if len(all) < latest {
		latest = len(all)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"featured": all[0],
		"latest":   all[:latest],
		"voting":   s.votes.IsRunning(all[0].ID),
		"models":   models.Catalog,
		"pipeline": s.votes.Status(),
	})
}

func (s *Server) handleAlignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	all, err := s.loadAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	result := alignment.Compute(all)
	recent := 5
	if len(all) < recent {
		recent = len(all)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"models":           result.Models,
		"bands":            result.Bands,
		"recentBills":      all[:recent],
		"billsScored":      result.BillsScored,
		"billsTotal":       len(all),
		"catalog":          models.Catalog,
		"contenderMatches": s.contenderMatches(ctx, all),
		"pipeline":         s.votes.Status(),
	})
}

// contenderMatches returns each model's closest sitting members of Congress on
// the 2028 watch list. The table is served from SQLite whenever the verdicts
// and floor votes behind it are the ones it was computed from, so a page load
// costs a lookup rather than a pass over the corpus.
func (s *Server) contenderMatches(ctx context.Context, bills []models.BillWithVotes) alignment.ContenderResult {
	inputs, err := s.store.ContenderInputs(ctx)
	signature := ""
	if err != nil {
		s.logger.Printf("alignment: reading the contender cache inputs: %v", err)
	} else {
		signature = alignment.Signature(inputs.ModelVotes, inputs.ModelVotesAt,
			inputs.MemberVotes, inputs.MemberVotesAt, s.minOverlap)
		if payload, err := s.store.CachedContenders(ctx, signature); err == nil {
			var cached alignment.ContenderResult
			if json.Unmarshal(payload, &cached) == nil {
				return cached
			}
		}
	}

	memberVotes, err := s.store.MemberVotes(ctx)
	if err != nil {
		s.logger.Printf("alignment: reading floor votes: %v", err)
	}
	result := alignment.MatchContenders(bills, memberVotes, s.minOverlap)
	result.Since = s.votes.Status().BillsSince

	if signature != "" {
		payload, err := json.Marshal(result)
		if err == nil {
			err = s.store.SaveContenders(ctx, signature, payload)
		}
		if err != nil {
			s.logger.Printf("alignment: caching the contender table: %v", err)
		}
	}
	return result
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"models": models.Catalog})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.votes.Status())
}

// handleVote queues a round rather than running one. A model reading a
// megabyte of statute section by section can take the better part of an hour,
// and no client is going to hold a connection open for that: the round happens
// in the background, and the reply is whatever is cached right now.
func (s *Server) handleVote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")

	bill, err := s.store.Bill(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "bill not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !s.votes.Status().VotingEnabled {
		writeError(w, http.StatusServiceUnavailable, "OPENROUTER_KEY is not set, so no votes can be collected")
		return
	}

	queued := s.votes.StartRound(id, r.URL.Query().Get("force") == "true")
	decorated, err := s.store.WithVotes(ctx, []models.Bill{bill})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"bill":   presentable(decorated)[0],
		"billId": id,
		"voting": true,
		// False when a round for this bill was already under way, in which case
		// this request joined it rather than starting a second one.
		"queued": queued,
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	started := s.votes.Refresh()
	writeJSON(w, http.StatusAccepted, map[string]any{
		"refreshing":     true,
		"alreadyRunning": !started,
	})
}

// loadAll returns every bill with its verdicts. The statutory text is dropped:
// it is only useful to the models, and it would dominate the list payloads.
func (s *Server) loadAll(ctx context.Context) ([]models.BillWithVotes, error) {
	bills, err := s.store.Bills(ctx)
	if err != nil {
		return nil, err
	}
	decorated, err := s.store.WithVotes(ctx, bills)
	if err != nil {
		return nil, err
	}
	for i := range decorated {
		decorated[i].FullText = ""
	}
	return presentable(decorated), nil
}

// presentable drops each summary's leading echo of its own bill title. Cleaning
// happens at ingest too, but rows written by earlier builds still carry the
// echo, so the last word on it belongs to the layer that serves them.
func presentable(bills []models.BillWithVotes) []models.BillWithVotes {
	for i := range bills {
		bills[i].Summary = models.SummaryWithoutTitleEcho(bills[i].Title, bills[i].Summary)
	}
	return bills
}

// withoutMemos drops the pros and cons from a listing payload. The browse table
// only renders vote glyphs, and five memos per row for a hundred rows is a lot
// of JSON nobody reads.
func withoutMemos(bills []models.BillWithVotes) []models.BillWithVotes {
	out := make([]models.BillWithVotes, len(bills))
	for i, b := range bills {
		votes := make([]models.Vote, len(b.Votes))
		for j, v := range b.Votes {
			v.Pros, v.Cons = nil, nil
			votes[j] = v
		}
		b.Votes = votes
		out[i] = b
	}
	return out
}

func matches(b models.BillWithVotes, q string) bool {
	for _, field := range []string{b.Number, b.Title, b.Summary, b.PolicyArea, b.Sponsor, b.Status} {
		if strings.Contains(strings.ToLower(field), q) {
			return true
		}
	}
	// "s1264" and "s 1264" should both find S. 1264.
	compact := strings.NewReplacer(".", "", " ", "").Replace(strings.ToLower(b.Number))
	return strings.Contains(compact, strings.NewReplacer(".", "", " ", "").Replace(q))
}

func hasVote(b models.BillWithVotes, modelKey, vote string) bool {
	for _, v := range b.Votes {
		if v.ModelKey == modelKey {
			return v.Vote == vote
		}
	}
	return false
}

func statusOptions(bills []models.BillWithVotes) []string {
	present := map[string]bool{}
	for _, b := range bills {
		if b.StatusCategory != "" {
			present[b.StatusCategory] = true
		}
	}
	var out []string
	for _, s := range models.StatusCategories {
		if present[s] {
			out = append(out, s)
		}
	}
	return out
}

func intParam(r *http.Request, key string, fallback int) int {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func logRequests(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if strings.HasPrefix(r.URL.Path, "/api/") || rec.status >= 400 {
			logger.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Millisecond))
		}
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
