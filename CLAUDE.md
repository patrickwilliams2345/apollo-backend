# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project status

Archived backend for the Apollo Reddit iOS app — shut down June 30, 2023 after Reddit's API pricing changes. No longer deployed. Code is preserved for historical reference; changes are unlikely to ship anywhere.

## Commands

- `make build` — compile the single `apollo` binary (`./cmd/apollo`).
- `make test` — runs `go test -race -timeout 1s ./...`. Tests that need Postgres skip themselves if `DATABASE_URL` is unset; `make test-setup` runs the migrations against `DATABASE_URL` to prepare a local DB.
- `make lint` — runs `golangci-lint` with the linters listed in `.golangci.yml` (notably `paralleltest`, `errcheck`, `sqlclosecheck`, `rowserrcheck`, `gochecknoinits`).
- Single test: `go test -race ./internal/repository -run TestPostgresWatcher_Create`.
- Migrations use `golang-migrate`; files live in `migrations/`. `docs/schema.sql` is the consolidated schema CI loads instead of stepping through migrations.

## Runtime topology

A single binary, three cobra subcommands. Each one is deployed as a separate Render service (see `render.yaml`):

- `apollo api` — Gorilla mux HTTP server (default port 4000, `$PORT` overrides). Routes in `internal/api/api.go`. Handles device/account registration from the iOS app, watcher CRUD, live activity registration, IAP receipt validation.
- `apollo scheduler` — single-instance ticker. Every 5s it runs SQL of the form `UPDATE ... SET next_check_at = $next WHERE id IN (SELECT id ... WHERE next_check_at < $now FOR UPDATE SKIP LOCKED LIMIT N) RETURNING id`, then publishes the returned IDs onto an rmq queue. This atomic claim-and-reschedule is the core scheduling primitive — don't replace it with a SELECT-then-UPDATE.
- `apollo worker --queue <name> --consumers <n>` — pulls jobs from one rmq queue and processes them. Queues: `notifications`, `stuck-notifications`, `subreddits`, `trending`, `users`, `live-activities`. Each has a `New<Queue>Worker` constructor wired in `internal/cmd/worker.go`.

Side channels: every process exposes pprof on `localhost:6060`; the scheduler also serves `:8080` for health.

## Two Redis instances, on purpose

Configured via separate env vars (`REDIS_QUEUE_URL`, `REDIS_LOCKS_URL`) and built via `cmdutil.NewRedisQueueClient` / `NewRedisLocksClient`:

- **Queue Redis** — backs `github.com/adjust/rmq/v5`. `noeviction`.
- **Locks Redis** — holds short-lived `SET key NX EX` keys keyed like `locks:accounts:<reddit_account_id>` and `locks:live-activities:<apns_token>`. The scheduler loads a Lua script once (`evalScript` in `internal/cmd/scheduler.go`) that takes a batch of candidate IDs and returns only those it successfully acquired the lock for. This is what prevents a job from being processed twice when checks overlap (`NotificationCheckTimeout` is the lock TTL).

Postgres is reached through PgBouncer in transaction mode, so `cmdutil.NewDatabasePool` forces `pgx.QueryExecModeSimpleProtocol` — don't switch to the default extended protocol or prepared-statement caching will break under PgBouncer.

## Code layout

- `internal/domain/` — pure types and `Repository` interfaces. No DB code here. Domain-level constants like `NotificationCheckInterval`, `SubredditCheckInterval`, `StaleTokenThreshold` live here and govern scheduler cadence.
- `internal/repository/` — Postgres implementations of the domain interfaces, one file per aggregate (`postgres_account.go`, etc.). Use `pgxpool.Pool` directly; the `Connection` interface in `connection.go` exists so methods can accept either a pool or a transaction.
- `internal/api/` — HTTP handlers, one file per resource. `api.go` wires repositories, the Reddit client, and the APNs token into the handler struct and registers all routes.
- `internal/worker/` — one file per queue. Each worker constructs its own `reddit.Client` and APNs `token.Token` from env vars at startup (they aren't shared with the API process). `worker.go` only defines the `Worker` interface and `NewWorkerFn`.
- `internal/reddit/` — Reddit OAuth + API client. Tracks rate-limit headers (`x-ratelimit-*`) and backs off; `RequestRemainingBuffer = 50` is the soft floor it keeps in reserve. Errors are mapped via `defaultErrorMap` (401/403 → `ErrOauthRevoked`, 429 → `ErrTooManyRequests`).
- `internal/cmd/` — cobra command definitions; this is where the wiring (DB pool sizes, consumer counts, queue names) lives.
- `internal/cmdutil/` — process-startup helpers (logger, statsd, Redis, Postgres pool, rmq connection). All processes go through these so connection tuning is centralized.
- `internal/itunes/` — App Store receipt validation used by `/v1/receipt`.
- `internal/testhelper/` — `NewTestPgxConn` for repository tests; skips when `DATABASE_URL` is empty.

## Conventions worth knowing

- Repository tests use the `_test` package (enforced by `testpackage` linter) and must call `t.Parallel()` (enforced by `paralleltest`).
- `gochecknoinits` is on — don't add `init()` functions.
- Observability is wired everywhere: every process builds a `zap.Logger`, a Datadog statsd client, and an OpenTelemetry tracer via Honeycomb's launcher. New code paths in the request/job hot path should emit at least a statsd metric.
- Bugsnag wraps the HTTP handler (`bugsnag.Handler(a.Routes())`) and is configured globally in `cmd.Execute` when `BUGSNAG_API_KEY` is set.
- Worker consumer counts are sized off `--consumers`; the DB pool gets `consumers/16`, locks Redis `consumers/4`, queue Redis `consumers/16`. Keep those ratios in mind if you change pool tuning.
