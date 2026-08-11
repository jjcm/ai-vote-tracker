// Command server runs the AI Vote Tracker: a Go API over SQLite plus the
// static Web Components frontend.
package main

import (
	"context"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	aivotetracker "github.com/pwnies/ai-vote-tracker"
	"github.com/pwnies/ai-vote-tracker/internal/config"
	"github.com/pwnies/ai-vote-tracker/internal/congress"
	"github.com/pwnies/ai-vote-tracker/internal/httpapi"
	"github.com/pwnies/ai-vote-tracker/internal/openrouter"
	"github.com/pwnies/ai-vote-tracker/internal/store"
	"github.com/pwnies/ai-vote-tracker/internal/votes"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)
	if err := run(logger); err != nil {
		logger.Fatalf("fatal: %v", err)
	}
}

func run(logger *log.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	web, err := webFS(cfg.WebDir, logger)
	if err != nil {
		return err
	}

	st, discarded, err := store.OpenWithMigration(cfg.DatabasePath)
	if err != nil {
		return err
	}
	defer st.Close()
	if discarded > 0 {
		logger.Printf("schema migration dropped %d verdict(s) cast on bill summaries; the corpus will be re-read as statute text", discarded)
	}

	siteURL := cfg.SiteURL
	if siteURL == "" {
		siteURL = "http://" + cfg.Addr()
	}
	router := openrouter.New(cfg.OpenRouterURL, cfg.OpenRouterKey, siteURL, cfg.RequestTimeout).
		WithLogger(logger).
		WithContextBudget(cfg.ContextBudgetRatio, cfg.ContextTokens)
	cg := congress.New(cfg.CongressBaseURL, cfg.CongressAPIKey).WithLogger(logger)
	svc := votes.New(st, router, cg, cfg.BootstrapBills, logger)

	if cfg.ContextTokens > 0 {
		logger.Printf("MODEL_CONTEXT_TOKENS=%d overrides every model's context window", cfg.ContextTokens)
	}

	if !cfg.HasOpenRouter() {
		logger.Printf("warning: OPENROUTER_KEY is not set — the site will render but every verdict stays pending")
	}
	if !cfg.HasCongress() {
		logger.Printf("CONGRESS_API_KEY is not set — using the built-in seed corpus")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	bootstrapCtx, cancel := context.WithTimeout(ctx, cfg.BootstrapTimeout+60*time.Second)
	if err := svc.Bootstrap(bootstrapCtx, cfg.BootstrapTimeout); err != nil {
		logger.Printf("bootstrap: %v", err)
	}
	cancel()

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           httpapi.New(st, svc, web, logger).Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Printf("AI Vote Tracker listening on http://%s", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		logger.Printf("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// webFS serves the embedded copy of web/ by default. Setting WEB_DIR serves
// from disk instead, which is handy while iterating on the frontend.
func webFS(dir string, logger *log.Logger) (fs.FS, error) {
	if dir != "" {
		if _, err := os.Stat(dir); err != nil {
			return nil, err
		}
		logger.Printf("serving static files from %s", dir)
		return os.DirFS(dir), nil
	}
	return aivotetracker.Web()
}
