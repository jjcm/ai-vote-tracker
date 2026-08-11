# AI Vote Tracker

How five frontier language models would vote on the bills before the United States
Congress: a binary **Yes** or **No** on final passage, plus one sentence of reasoning
from each model.

Built by [Jacob Miller (@pwnies)](https://x.com/pwnies). Design created with
[diffui.ai](https://diffui.ai) — the reference renders live in `assets/design/`.

## Run it

```bash
cp .env.example .env      # then set OPENROUTER_KEY
go run ./cmd/server
# open http://127.0.0.1:8400
```

On first start the server loads a corpus of bills, votes the newest one
synchronously so the homepage is never blank, and collects the remaining verdicts in
the background. Everything is cached in SQLite, so OpenRouter is billed once per
(bill, model) pair rather than once per page load.

## Configuration

Read from `.env` (gitignored) or the process environment.

| Variable | Default | Purpose |
| --- | --- | --- |
| `OPENROUTER_KEY` | *(required for verdicts)* | OpenRouter API key. Without it the site still runs, but every verdict stays pending. |
| `PORT` | `8400` | HTTP port. |
| `HOST` | `127.0.0.1` | Bind address. |
| `CONGRESS_API_KEY` | *(optional)* | [Congress.gov API](https://api.congress.gov/sign-up/) key. Without it the built-in sample corpus is used. |
| `OPENROUTER_BASE_URL` | `https://openrouter.ai/api/v1` | Override for local testing. |
| `CONGRESS_BASE_URL` | `https://api.congress.gov/v3` | Override for local testing. |
| `DATABASE_PATH` | `data/aivotes.db` | SQLite file. Delete it to re-seed and re-vote. |
| `BOOTSTRAP_BILLS` | `12` | How many live bills to pull from Congress.gov. |
| `WEB_DIR` | *(unset)* | Serve `web/` from disk instead of the embedded copy, for frontend iteration. |
| `MODEL_TIMEOUT_SECONDS` | `90` | Per-model request timeout. |
| `BOOTSTRAP_TIMEOUT_SECONDS` | `120` | How long startup waits for the featured bill's verdicts. |

Never commit `.env`. It is gitignored, and `.env.example` carries the shape without
any secrets.

## The models

Votes are collected from these five over OpenRouter, in parallel per bill:

| Display name | OpenRouter ID |
| --- | --- |
| GPT Sol | `openai/gpt-5.6-sol` |
| Opus | `anthropic/claude-opus-5` |
| Grok | `x-ai/grok-4.5` |
| DeepSeek | `deepseek/deepseek-v4-pro` |
| Gemini | `google/gemini-3.6-flash` |

Each model is sent the bill number, title, chamber, latest action, summary, and an
excerpt of the statutory text, and is asked for
`{"vote": "Yes" | "No", "reason": "<one sentence>"}`. There is no abstain option.
Replies that arrive fenced, prefixed, or in prose are salvaged where possible;
anything that cannot be reduced to a binary verdict is recorded as an error and
retried on the next round rather than shown as a vote.

## Pages

| Route | Contents |
| --- | --- |
| `/` | "How would AI vote?" — the featured bill with all five verdicts, plus the latest bills table. |
| `/bills` | Full listing with keyword search, chamber / model / status filters, and pagination. |
| `/alignment` | Each model's position on a −1.0 to +1.0 spectrum, model snapshots, methodology, and a recent-bill agreement grid. |
| `/about` | Colophon. |

## API

| Endpoint | Purpose |
| --- | --- |
| `GET /api/featured` | Featured bill plus the latest eight, with verdicts. |
| `GET /api/bills` | Filtered, paginated list. Query: `q`, `chamber`, `status`, `model`, `vote`, `page`, `perPage`. |
| `GET /api/bills/{id}` | One bill with its full text and verdicts. |
| `POST /api/bills/{id}/vote` | Re-run the models for a bill. Add `?force=true` to overwrite existing verdicts; otherwise only missing ones are collected. Returns `202` if the round outlives the request. |
| `GET /api/alignment` | Computed alignment, score bands, and recent bills. |
| `GET /api/models` | The model catalog. |
| `GET /api/status` | Data source, whether voting is enabled, rounds in flight. |
| `POST /api/refresh` | Re-read the upstream bill list and vote anything new. |

## How alignment is computed

Each bill carries an ideology score from −1.0 (progressive) to +1.0 (conservative).
For live bills the score is assigned by a model at ingest; for the sample corpus it
is set by hand in `internal/seed`. A model's alignment is the ideology-weighted
average of its votes — a Yes moves it toward the bill's score, a No moves it away —
so a model that consistently backs conservative bills lands near +1.0. Bills without
a score contribute nothing.

## Bill sources

With `CONGRESS_API_KEY` set, bills come from the Congress.gov `/summaries` feed
(newest first, House and Senate bills only, ceremonial resolutions skipped), with a
follow-up call per bill for the latest action, policy area, and sponsor. Without a
key the server falls back to a corpus of twelve realistic sample bills that carry
enough statutory text for the models to reason about — the votes on them are still
real model output.

## Layout

```
cmd/server        HTTP server entrypoint
cmd/mockrouter    development-only OpenRouter stub (fake verdicts, no API key needed)
internal/config   .env and environment loading
internal/models   domain types and the model catalog
internal/store    SQLite persistence
internal/seed     offline bill corpus
internal/congress Congress.gov client
internal/openrouter  chat completions client and response parsing
internal/votes    bootstrap, parallel voting rounds, refresh
internal/alignment   spectrum computation
web/              static site: HTML, CSS tokens, and native Web Components
assets/design/    the reference renders this UI was built against
```

The frontend has no build step. `web/js/components/` holds native custom elements —
`site-header`, `vote-badge`, `model-vote-card`, `featured-bill`, `latest-bills`,
`bills-browser`, `alignment-report`, `model-roster` — and the server embeds `web/`
into the binary, so `go build ./cmd/server` produces a single self-contained
executable.

## Development

```bash
go build ./...
go test ./...

# Exercise the full pipeline without an OpenRouter key or a bill.
go run ./cmd/mockrouter &
OPENROUTER_KEY=dev OPENROUTER_BASE_URL=http://127.0.0.1:8500/api/v1 \
  WEB_DIR=./web go run ./cmd/server

# Capture screenshots of every route to compare against assets/design/.
./shot.sh
```

`cmd/mockrouter` returns deterministic fake verdicts derived from a hash of the bill
and model name. It exists to test plumbing; nothing it produces is a real model
opinion.
