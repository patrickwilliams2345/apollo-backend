# CLAUDE.md

Codice stato: ST:F1/5:web-session-auth

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

Self-hosting fork of the archived `christianselig/apollo-backend`. The original backend was Apollo's production push-notification + watcher service, shut down June 30, 2023 after Reddit's API pricing changes. This fork is being adapted for single-tenant self-hosting against sideloaded Apollo builds (e.g. via JeffreyCA's `Apollo-ImprovedCustomApi` tweak). Christian-specific integrations (App Store IAP, Live Activities, Bugsnag, SMTP2GO, Render) have been stripped.

## Commands

- `make build` — compile the single `apollo` binary (`./cmd/apollo`).
- `make test` — runs `go test -race -timeout 1s ./...`. Tests that need Postgres skip themselves if `DATABASE_URL` is unset; `make test-setup` runs the migrations against `DATABASE_URL` to prepare a local DB.
- `make lint` — runs `golangci-lint` with the linters listed in `.golangci.yml` (notably `paralleltest`, `errcheck`, `sqlclosecheck`, `rowserrcheck`, `gochecknoinits`).
- Single test: `go test -race ./internal/repository -run TestPostgresWatcher_Create`.
- Migrations use `golang-migrate`; files live in `migrations/`. `docs/schema.sql` is the consolidated schema CI loads instead of stepping through migrations.

## Runtime topology

A single binary, three cobra subcommands. Each is deployed as a separate container (see `docker-compose.yml`):

- `apollo api` — Gorilla mux HTTP server (default port 4000, `$PORT` overrides). Routes in `internal/api/api.go`. Handles device/account registration from the iOS app and watcher CRUD.
- `apollo scheduler` — single-instance ticker. Every 5s it runs SQL of the form `UPDATE ... SET next_check_at = $next WHERE id IN (SELECT id ... WHERE next_check_at < $now FOR UPDATE SKIP LOCKED LIMIT N) RETURNING id`, then publishes the returned IDs onto an rmq queue. This atomic claim-and-reschedule is the core scheduling primitive — don't replace it with a SELECT-then-UPDATE.
- `apollo worker --queue <name> --consumers <n>` — pulls jobs from one rmq queue and processes them. Queues: `notifications`, `stuck-notifications`, `subreddits`, `trending`, `users`. Each has a `New<Queue>Worker` constructor wired in `internal/cmd/worker.go`.

Side channels: every process exposes pprof on `localhost:6060`; the scheduler also serves `:8080` for health.

## Two Redis instances, on purpose

Configured via separate env vars (`REDIS_QUEUE_URL`, `REDIS_LOCKS_URL`) and built via `cmdutil.NewRedisQueueClient` / `NewRedisLocksClient`:

- **Queue Redis** — backs `github.com/adjust/rmq/v5`. `noeviction`.
- **Locks Redis** — holds short-lived `SET key NX EX` keys keyed like `locks:accounts:<reddit_account_id>`. The scheduler loads a Lua script once (`evalScript` in `internal/cmd/scheduler.go`) that takes a batch of candidate IDs and returns only those it successfully acquired the lock for. This is what prevents a job from being processed twice when checks overlap (`NotificationCheckTimeout` is the lock TTL).

Postgres is reached through PgBouncer in transaction mode, so `cmdutil.NewDatabasePool` forces `pgx.QueryExecModeSimpleProtocol` — don't switch to the default extended protocol or prepared-statement caching will break under PgBouncer.

## Code layout

- `internal/domain/` — pure types and `Repository` interfaces. No DB code here. Domain-level constants like `NotificationCheckInterval`, `SubredditCheckInterval`, `StaleTokenThreshold` live here and govern scheduler cadence.
- `internal/repository/` — Postgres implementations of the domain interfaces, one file per aggregate (`postgres_account.go`, etc.). Use `pgxpool.Pool` directly; the `Connection` interface in `connection.go` exists so methods can accept either a pool or a transaction.
- `internal/api/` — HTTP handlers, one file per resource. `api.go` wires repositories, the Reddit client, and the APNs token into the handler struct and registers all routes.
- `internal/worker/` — one file per queue. Each worker constructs its own `reddit.Client` and APNs `token.Token` from env vars at startup (they aren't shared with the API process). `worker.go` only defines the `Worker` interface and `NewWorkerFn`.
- `internal/reddit/` — Reddit OAuth + API client. Tracks rate-limit headers (`x-ratelimit-*`) and backs off; `RequestRemainingBuffer = 50` is the soft floor it keeps in reserve. Errors are mapped via `defaultErrorMap` (401/403 → `ErrOauthRevoked`, 429 → `ErrTooManyRequests`).
- `internal/cmd/` — cobra command definitions; this is where the wiring (DB pool sizes, consumer counts, queue names) lives.
- `internal/cmdutil/` — process-startup helpers (logger, statsd, Redis, Postgres pool, rmq connection). All processes go through these so connection tuning is centralized.
- `internal/testhelper/` — `NewTestPgxConn` for repository tests; skips when `DATABASE_URL` is empty.

## Conventions worth knowing

- Repository tests use the `_test` package (enforced by `testpackage` linter) and must call `t.Parallel()` (enforced by `paralleltest`).
- `gochecknoinits` is on — don't add `init()` functions.
- Observability is opt-in: every process builds a `zap.Logger`, a statsd client (`statsd.ClientInterface` — a `NoOpClient` when `STATSD_URL` is unset), and an OpenTelemetry tracer via Honeycomb's launcher (also no-op without env vars). New code paths in the request/job hot path should still emit a statsd metric — it's free when disabled.
- Worker consumer counts are sized off `--consumers`; the DB pool gets `consumers/16`, locks Redis `consumers/4`, queue Redis `consumers/16`. Keep those ratios in mind if you change pool tuning.

## Single-tenant self-hosting model

- One deployment serves one sideloaded Apollo build. The APNs topic is configured per-deployment via `APPLE_APNS_TOPIC` (the build's bundle ID); `cmdutil.APNSTopic()` reads it and panics on startup if unset.
- Reddit OAuth credentials are **per-account**, not per-deployment. Each row in `accounts` carries its own `reddit_client_id` / `reddit_client_secret` / `reddit_user_agent` / `reddit_redirect_uri`. The tweak sends these at registration time. The process-level `reddit.Client` carries no credentials; `AuthenticatedClient` holds them and `RefreshTokens` uses Basic auth with the per-account client_id/secret.
- Registration endpoints (`POST /v1/device`, `POST /v1/device/{apns}/account`, `POST /v1/device/{apns}/accounts`) are gated by `REGISTRATION_SECRET` when set — clients must send it as `X-Registration-Token`. When unset, registration is open (intended for local dev or private-network instances).
- The iOS-app-facing registration payload is the explicit `accountRegistrationRequest` struct in `internal/api/accounts.go`, not `domain.Account` directly — counters and DB IDs are not user-controlled.


<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:6cd5cc61 -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

**Architecture in one line:** issues live in a local Dolt DB; sync uses `refs/dolt/data` on your git remote; `.beads/issues.jsonl` is a passive export. See https://github.com/gastownhall/beads/blob/main/docs/SYNC_CONCEPTS.md for details and anti-patterns.

## Agent Context Profiles

The managed Beads block is task-tracking guidance, not permission to override repository, user, or orchestrator instructions.

- **Conservative (default)**: Use `bd` for task tracking. Do not run git commits, git pushes, or Dolt remote sync unless explicitly asked. At handoff, report changed files, validation, and suggested next commands.
- **Minimal**: Keep tool instruction files as pointers to `bd prime`; use the same conservative git policy unless active instructions say otherwise.
- **Team-maintainer**: Only when the repository explicitly opts in, agents may close beads, run quality gates, commit, and push as part of session close. A current "do not commit" or "do not push" instruction still wins.

## Session Completion

This protocol applies when ending a Beads implementation workflow. It is subordinate to explicit user, repository, and orchestrator instructions.

1. **File issues for remaining work** - Create beads for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **Handle git/sync by active profile**:
   ```bash
   # Conservative/minimal/default: report status and proposed commands; wait for approval.
   git status

   # Team-maintainer opt-in only, unless current instructions forbid it:
   git pull --rebase
   git push
   git status
   ```
5. **Hand off** - Summarize changes, validation, issue status, and any blocked sync/commit/push step

**Critical rules:**
- Explicit user or orchestrator instructions override this Beads block.
- Do not commit or push without clear authority from the active profile or the current user request.
- If a required sync or push is blocked, stop and report the exact command and error.
<!-- END BEADS INTEGRATION -->
