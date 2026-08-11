# AI Vote Tracker

How five frontier language models would vote on the bills before the United States
Congress: a binary **Yes** or **No** on final passage, plus one sentence of reasoning
from each model.

Every verdict is cast on the **text of the law**. Each model is given the bill's own
statutory text — the XML Congress.gov publishes — writes its own pros and cons memo
from it, and then votes on that memo. No model is shown a CRS summary, a committee
report, or another model's notes.

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
| `BOOTSTRAP_TIMEOUT_SECONDS` | `240` | How long startup waits for the featured bill's verdicts. A round is at least two calls per model, and more when a bill has to be digested. |
| `CONTEXT_BUDGET_RATIO` | `0.75` | Share of a model's context window that statute text may fill before the bill is read section by section. |
| `MODEL_CONTEXT_TOKENS` | *(unset)* | Overrides every model's context window. For exercising the section-digest path without an omnibus-sized bill. |

Never commit `.env`. It is gitignored, and `.env.example` carries the shape without
any secrets.

## The models

Votes are collected from these five over OpenRouter, in parallel per bill. The
context window is what decides whether a model can read a bill in one pass:

| Display name | OpenRouter ID | Context window |
| --- | --- | --- |
| GPT Sol | `openai/gpt-5.6-sol` | 1,050,000 |
| Opus | `anthropic/claude-opus-5` | 1,000,000 |
| Grok | `x-ai/grok-4.5` | 500,000 |
| DeepSeek | `deepseek/deepseek-v4-pro` | 1,048,576 |
| Gemini | `google/gemini-3.6-flash` | 1,048,576 |

## How a verdict is reached

Each model runs the same three stages, and runs all of them itself:

1. **Section notes**, only when needed. If the bill's text is larger than
   `CONTEXT_BUDGET_RATIO` of that model's window, the bill is split on its own
   structural boundaries — `section` first, then `title`, `subtitle`, `part`,
   `division`, falling back to `SEC. n.` headings and then to paragraphs — and the
   model summarizes each piece into a note. Tokens are estimated conservatively at
   four characters each. The overflow is logged with the bill, the model, the size
   of the text and the budget it broke.
2. **A pros and cons memo**, always. The model argues both sides from the statute
   text, or from its own section notes, and returns structured `pros` and `cons`.
3. **The vote**. Yes or No on final passage, with one sentence of reasoning, decided
   from the text and that model's own memo. There is no abstain option.

Notes and memos are private to the model that wrote them: no model ever reads
another's. Memos are cached against a fingerprint of the text they were written
from, so a re-vote reuses a model's own reasoning, and newly published text
discards it.

Replies rarely arrive clean. The parser handles markdown fences, chatter on either
side of the object, prose answers with no JSON at all, memo arrays that stop after
their third entry, and objects that stop mid-string when a reasoning model spends
its token budget on hidden thinking. A truncated reply is retried with double the
budget before it is given up on, and a rationale that still carries the response
envelope is discarded rather than printed. Anything that cannot be reduced to a
binary verdict with a readable sentence is recorded as an error and retried on the
next round rather than shown as a vote.

On startup the server also drops any stored rationale that looks like leaked JSON,
so verdicts written by an older build are re-collected rather than left on the page.

## Pages

| Route | Contents |
| --- | --- |
| `/` | "How would AI vote?" — the featured bill with all five verdicts, each with the model's own pros and cons a click away, plus the latest bills table. |
| `/bills` | Full listing with keyword search, chamber / model / status filters, and pagination. |
| `/alignment` | Each model's position on a −1.0 to +1.0 spectrum, model snapshots, methodology, and a recent-bill agreement grid. |
| `/about` | Colophon. |

## API

| Endpoint | Purpose |
| --- | --- |
| `GET /api/featured` | Featured bill plus the latest eight, with verdicts. |
| `GET /api/bills` | Filtered, paginated list. Query: `q`, `chamber`, `status`, `model`, `vote`, `page`, `perPage`. |
| `GET /api/bills/{id}` | One bill with its verdicts, each carrying the `pros` and `cons` that model wrote, and the size of the statute text. Add `?text=true` for the text itself, which for a live bill is megabytes of XML. |
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

With `CONGRESS_API_KEY` set, the Congress.gov `/summaries` feed names the bills that
have moved recently (newest first, House and Senate bills only, ceremonial
resolutions skipped). For each one the server then reads
`/bill/{congress}/{type}/{number}/text` and downloads the **Formatted XML** of the
newest text version available, falling back to the **Formatted Text** print — and
logging that it did — when a bill has no XML. Congress publishes a bill's text some
days after introduction, so a bill with no printed text yet is skipped rather than
voted on from its summary; the date window widens until enough bills with text come
back. If the whole fetch fails, the server falls back to the offline corpus.

The CRS summary is still stored, because the bill cards read better with it. It is
not shown to any model.

Without a key the server uses a corpus of thirteen sample bills written as bill XML
in the same shape Congress.gov publishes, including an appropriations act long
enough to divide into account-level sections. The votes on them are still real model
output. `MODEL_CONTEXT_TOKENS=16000` shrinks every window enough that the
appropriations act overflows and the digest path runs offline.

The database records where a bill's text came from (`textSource`, `textFormat`,
`textVersion`, `textUrl`). Schema version 2 clears a database written by the
summary-era build once, so those bills are re-read as statute text and re-voted.

## Layout

```
cmd/server        HTTP server entrypoint
cmd/mockrouter    development-only OpenRouter stub (fake verdicts, no API key needed)
internal/config   .env and environment loading
internal/models   domain types and the model catalog
internal/store    SQLite persistence
internal/seed     offline bill corpus, written as bill XML
internal/billtext token estimation and structural splitting of statute text
internal/congress Congress.gov client, including bill text downloads
internal/openrouter  chat completions client, the deliberation pipeline, response parsing
internal/votes    bootstrap, parallel deliberation and voting rounds, refresh
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

# Exercise the full pipeline without an OpenRouter key or a bill. The context
# override shrinks every window so the section-digest path runs too.
go run ./cmd/mockrouter &
OPENROUTER_KEY=dev OPENROUTER_BASE_URL=http://127.0.0.1:8500/api/v1 \
  MODEL_CONTEXT_TOKENS=16000 WEB_DIR=./web go run ./cmd/server

# Capture screenshots of every route to compare against assets/design/.
./shot.sh
```

`cmd/mockrouter` returns deterministic fake verdicts derived from a hash of the bill
and model name. It exists to test plumbing; nothing it produces is a real model
opinion.
