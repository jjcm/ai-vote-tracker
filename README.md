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

On first start the server loads a corpus of bills, starts serving immediately, and
collects the verdicts in the background — newest bill first, so the homepage fills
in from the top. Verdicts arrive as they are decided; until then the page shows them
as pending and polls.

Nobody ever waits on a model. Every verdict is collected once per (bill, model),
cached in SQLite, and served from there, so the pipeline is free to be as slow as
being thorough requires: no request blocks on it, no stage is skipped for speed, and
a bill that takes an hour to work through takes an hour.

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
| `CONGRESS_USER_AGENT` | `AIVoteTracker/1.0 (…)` | What to call this client when talking to Congress.gov, the House Clerk and the Senate. Put contact details here if you like; leaving it blank keeps the built-in value rather than sending nothing. |
| `DATABASE_PATH` | `data/aivotes.db` | SQLite file. Delete it to re-seed and re-vote. |
| `BILLS_SINCE` | `2026-06-01` | Start of the analysis window. Bills, verdicts and congressional floor votes all cover legislation that has moved on or after this date. |
| `BOOTSTRAP_BILLS` | `40` | How many live bills to pull from Congress.gov. |
| `CONTENDER_MIN_OVERLAP` | `3` | How many bills a model and a member of Congress must both have taken a side on before the pair is ranked. |
| `WEB_DIR` | *(unset)* | Serve `web/` from disk instead of the embedded copy, for frontend iteration. |
| `MODEL_TIMEOUT_SECONDS` | `300` | Per-request timeout. Generous on purpose: a timeout here throws away a whole deliberation, and nothing is waiting on it. |
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
from, so filling in a verdict that failed earlier reuses the model's own reasoning
rather than improvising a new one, while newly published text discards it. Asking
for a re-run with `?force=true` starts every model over from the text.

## When the work happens

None of this is on the request path:

- **Startup** loads the corpus and hands the verdicts to a background backfill. The
  server starts listening straight away, so a slow provider delays verdicts rather
  than the site.
- **The backfill** works through the bills that need verdicts one at a time, newest
  first, then sweeps the corpus twice more with a pause between passes, so a model
  that was rate limited or cut off gets another turn instead of leaving an error on
  the page. Serial per bill keeps a long run readable in the log; within a bill the
  five models run in parallel.
- **`POST /api/bills/{id}/vote`** queues a round and answers `202` immediately with
  the cached state. The page polls.
- **`POST /api/refresh`** returns before it has even read the bill list.

The read endpoints only ever touch SQLite, so they answer in microseconds whether or
not a round is running.

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
| `/bills` | Full listing with keyword search, chamber / model / status filters, and pagination. Clicking a row opens that bill. |
| `/bills/{id}` | One bill, presented as the homepage card opened out: sponsor, policy area, stage, a link to Congress.gov, and the five verdicts with each model's pros and cons. |
| `/alignment` | Each model's position on a −1.0 to +1.0 spectrum, model snapshots, the sitting members of Congress floated for 2028 that each model votes with most often, methodology, and a recent-bill agreement grid. |
| `/about` | Colophon. |

## API

| Endpoint | Purpose |
| --- | --- |
| `GET /api/featured` | Featured bill plus the latest eight, with verdicts. |
| `GET /api/bills` | Filtered, paginated list. Query: `q`, `chamber`, `status`, `model`, `vote`, `page`, `perPage`. |
| `GET /api/bills/{id}` | One bill with its verdicts, each carrying the `pros` and `cons` that model wrote, and the size of the statute text. Add `?text=true` for the text itself, which for a live bill is megabytes of XML. |
| `POST /api/bills/{id}/vote` | Queue a round for a bill and return `202` with whatever is cached now; the round runs in the background. Add `?force=true` to start every model over from a blank page — fresh notes, fresh memo, fresh vote — instead of collecting only the missing verdicts. `queued` is false when a round was already under way. |
| `GET /api/alignment` | Computed alignment, score bands, recent bills, and `contenderMatches`: each model's closest members of Congress, the watch list itself, and the frequently named potentials who hold no seat. |
| `GET /api/models` | The model catalog. |
| `GET /api/status` | Data source, whether voting is enabled, rounds in flight, whether a backfill is running. |
| `POST /api/refresh` | Re-read the upstream bill list and vote anything new, in the background. Returns `202` at once; `alreadyRunning` is true when a refresh was already going. |

## How alignment is computed

Each bill carries an ideology score from −1.0 (progressive) to +1.0 (conservative).
For live bills the score is assigned by a model at ingest; for the sample corpus it
is set by hand in `internal/seed`. A model's alignment is the ideology-weighted
average of its votes — a Yes moves it toward the bill's score, a No moves it away —
so a model that consistently backs conservative bills lands near +1.0. Bills without
a score contribute nothing.

## The closest 2028 contenders

The alignment page also asks a narrower question: of the people being floated
for 2028 who can actually cast a vote, which one does each model vote like?

**Who is on the list.** A curated set of sitting senators and representatives
in `internal/contenders`. It is editorial and says so: there is no registry to
read, because nobody has filed with the FEC, so the standard is *frequently
mentioned as a 2028 presidential potential in national press as of 2026* — the
recurring names in cycle handicapping and shortlist coverage. It lives in code
so that adding or dropping a name is a diff somebody can argue with.

Governors, the Vice President and cabinet officers come up as often as anyone
on it and cast no floor votes, so there is nothing to compare a model against.
They are named on the page under "named for 2028, but not in Congress" and
carry no score. Nothing here is invented for them.

**Where the votes come from.** The House Clerk (`clerk.house.gov/evs`) and the
Senate (`senate.gov/legislative/LIS`), read directly as XML. Congress.gov,
which the rest of this project uses, exposes House recorded votes through a
beta `/v3/house-vote` endpoint and has no Senate equivalent at all, so a
Congress.gov-only implementation could reach Ocasio-Cortez and Massie but not
one senator — and the Senate is where almost every 2028 potential who can vote
sits. Both chambers publish complete per-member roll calls with no key, the
House stamping each legislator with a Bioguide ID and the Senate with an LIS
member ID. Both sit behind the same CDN as Congress.gov, so every request is
stamped with a user agent by its transport.

Only votes on **final passage** count — `On Passage`, a suspension of the
rules, concurring in the other chamber's amendment, overriding a veto. Cloture,
motions to proceed, recommittals and amendments are votes about a bill's path
rather than about enacting it, and a model that read the statute text and
answered "would you pass this" has no opinion on them.

**The arithmetic.** For each model and each member, over the bills where *both*
took a binary side — the model returned Yes or No and the member is recorded
Yea or Nay — the agreement rate is the share they landed on the same side of.
Present, absent, and a verdict that never arrived leave the pair nothing to
compare, so they come out of the denominator rather than counting as a
disagreement. Below `CONTENDER_MIN_OVERLAP` shared votes a pair is reported but
not ranked: one shared vote makes anybody a hundred per cent match.

Ranking is not on the raw rate. The House takes dozens of final-passage votes
in a session and the Senate takes a handful, so a senator is routinely three
for three with a model while a representative is thirty of thirty-three, and
ordering by rate alone hands every headline match to whoever has the least
evidence behind it. The order is by the lower bound of the Wilson interval on
the pair's agreement, which discounts a rate for how thin it is. The percentage
the page prints is still the plain one.

Member positions are cached in SQLite, keyed by bill and member so a bill voted
on twice keeps the later position, and a roll call already read is never
downloaded again. The agreement table is cached too, against a signature of the
verdicts and floor votes it was computed from plus a version for the code that
computed it, so a page load is a lookup until one of them moves. All of it runs
in the background on startup, and none of it needs an API key.

## Bill sources

The corpus covers the analysis window that starts at `BILLS_SINCE`, and within
it the bills that reached a recorded vote on final passage come first: those
are the ones where a model's verdict can be set against a member of Congress's.
The `/summaries` feed then tops the list up to `BOOTSTRAP_BILLS`.

With `CONGRESS_API_KEY` set, the Congress.gov `/summaries` feed names the bills that
have moved in the window (newest first, House and Senate bills only, ceremonial
resolutions skipped). For each one the server then reads
`/bill/{congress}/{type}/{number}/text` and downloads the **Formatted XML** of the
newest text version available, falling back to the **Formatted Text** print — and
logging that it did — when a bill has no XML. Congress publishes a bill's text some
days after introduction, so a bill with no printed text yet is skipped rather than
voted on from its summary; the date window widens until enough bills with text come
back. If the whole fetch fails, the server falls back to the offline corpus.

The CRS summary is still stored, because the bill cards read better with it. It is
not shown to any model.

Both hosts are behind a CDN that will not serve an anonymous client: a text
download with no `User-Agent` gets a 403 from `www.congress.gov`, and a request
with a default library agent can draw a Cloudflare 1010 from `api.congress.gov`
depending on the network it comes from. Every request the client makes is
therefore stamped with a descriptive agent by its transport, downloads included,
so no call site can omit one. `CONGRESS_USER_AGENT` replaces it.

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
internal/rollcall House Clerk and Senate roll calls, read as XML
internal/contenders  the curated 2028 watch list, and who is on it but not in Congress
internal/openrouter  chat completions client, the deliberation pipeline, response parsing
internal/votes    bootstrap, parallel deliberation and voting rounds, refresh
internal/alignment   spectrum computation and model-to-member agreement
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
