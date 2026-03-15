# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
make build          # Build binary to bin/ddarabot
make test           # Run all tests (go test ./... -v)
make lint           # Check formatting (gofmt) + go vet
make fmt            # Auto-format code
make run            # Run with config.toml
make docker-build   # Build Docker image
make docker-deploy  # Deploy with docker compose
```

Run a single test:
```bash
go test -v -run TestFunctionName ./internal/package/
```

## Architecture

DDaraBot is a Bluesky auto-translation bot. It monitors a user's posts via Jetstream WebSocket, translates them using Genkit LLM, and posts translated replies from the same account.

**Data flow:** Jetstream → filter (hashtag trigger) → translator (Genkit LLM) → poster (XRPC reply)

### Key packages (`internal/`)

- **bot** — Main event loop: subscribes to Jetstream events, orchestrates filter → translate → post pipeline. Uses goroutines with semaphore pattern for concurrent posting.
- **jetstream** — WebSocket client connecting to Bluesky Jetstream firehose. Emits `Post` events filtered by DID and collection.
- **translator** — Genkit-based translation with retry (3 attempts). Summarizes text if it exceeds 300 graphemes.
- **bluesky** — XRPC HTTP client. `Auth` handles JWT session create/refresh. `Poster` creates reply records via `com.attroto.repo.createRecord`.
- **filter** — Detects trigger hashtag in post facets, filters out replies.
- **config** — TOML config parsing with env var overrides (`DDARA_BLUESKY_APP_PASSWORD`, `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GOOGLE_API_KEY`).
- **store** — BoltDB (bbolt) for cursor persistence and processed post deduplication.

### Entry point

`cmd/ddarabot/main.go` — CLI with subcommands: run (default), `validate`, `version`. Supports `--config` and `--dry-run` flags.

## Code Style

- Format with `gofmt` (enforced in CI)
- All commit messages and code comments must be in English
- Error wrapping: use `fmt.Errorf("context: %w", err)`
- Dependency injection via interfaces (`Auth`, `Poster`)
- Structured logging with `slog`
- Context-aware functions throughout

## LLM Provider Convention

Model strings use `provider/model` format (e.g., `googleai/gemini-2.5-flash`). The provider prefix determines which Genkit plugin is initialized.
