// Package config loads runtime configuration from .env files and environment
// variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config holds every runtime knob the server understands.
type Config struct {
	Host            string
	Port            int
	OpenRouterKey   string
	OpenRouterURL   string
	CongressAPIKey  string
	CongressBaseURL string
	// CongressUserAgent identifies this client to Congress.gov, which blocks
	// requests that do not identify themselves. Blank keeps the built-in one.
	CongressUserAgent string
	DatabasePath      string
	WebDir            string
	SiteURL           string
	BootstrapBills    int
	RequestTimeout    time.Duration
	// ContextBudgetRatio is the share of a model's context window that statute
	// text may fill before the bill is read section by section instead.
	ContextBudgetRatio float64
	// ContextTokens overrides every model's context window. It exists to
	// exercise the section-digest path without an omnibus-sized bill.
	ContextTokens int
}

// Load reads .env (if present) and then the process environment.
func Load() (*Config, error) {
	// A missing .env is fine: real deployments use plain environment variables.
	_ = godotenv.Load()

	cfg := &Config{
		Host:            envStr("HOST", "127.0.0.1"),
		Port:            envInt("PORT", 8400),
		OpenRouterKey:   strings.TrimSpace(os.Getenv("OPENROUTER_KEY")),
		OpenRouterURL:   envStr("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1"),
		CongressAPIKey:  strings.TrimSpace(os.Getenv("CONGRESS_API_KEY")),
		CongressBaseURL: envStr("CONGRESS_BASE_URL", "https://api.congress.gov/v3"),

		CongressUserAgent: strings.TrimSpace(os.Getenv("CONGRESS_USER_AGENT")),

		DatabasePath:   envStr("DATABASE_PATH", "data/aivotes.db"),
		WebDir:         strings.TrimSpace(os.Getenv("WEB_DIR")),
		SiteURL:        envStr("SITE_URL", ""),
		BootstrapBills: envInt("BOOTSTRAP_BILLS", 12),
		// Generous on purpose. A model digesting a section of a large bill is
		// slow, nothing is waiting on it, and a timeout here throws away a
		// whole deliberation.
		RequestTimeout: time.Duration(envInt("MODEL_TIMEOUT_SECONDS", 300)) * time.Second,

		ContextBudgetRatio: envFloat("CONTEXT_BUDGET_RATIO", 0.75),
		ContextTokens:      envInt("MODEL_CONTEXT_TOKENS", 0),
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("PORT must be between 1 and 65535, got %d", cfg.Port)
	}
	if cfg.ContextBudgetRatio <= 0 || cfg.ContextBudgetRatio > 1 {
		return nil, fmt.Errorf("CONTEXT_BUDGET_RATIO must be between 0 and 1, got %v", cfg.ContextBudgetRatio)
	}
	cfg.OpenRouterURL = strings.TrimRight(cfg.OpenRouterURL, "/")
	cfg.CongressBaseURL = strings.TrimRight(cfg.CongressBaseURL, "/")
	return cfg, nil
}

// HasOpenRouter reports whether model voting can run.
func (c *Config) HasOpenRouter() bool { return c.OpenRouterKey != "" }

// HasCongress reports whether live Congress.gov bills can be fetched.
func (c *Config) HasCongress() bool { return c.CongressAPIKey != "" }

// Addr is the listen address for the HTTP server.
func (c *Config) Addr() string { return fmt.Sprintf("%s:%d", c.Host, c.Port) }

func envStr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envFloat(key string, fallback float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
