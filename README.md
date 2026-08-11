# AI Vote Tracker

See how frontier LLMs would vote on US bills. Design via [DiffUI](https://diffui.ai). Built by [@pwnies](https://x.com/pwnies).

## Stack
- Go backend (`cmd/server`)
- Web Components frontend (`web/`)
- OpenRouter for model votes

## Setup
```bash
cp .env.example .env   # set OPENROUTER_KEY
go run ./cmd/server
# open http://127.0.0.1:8400
```
