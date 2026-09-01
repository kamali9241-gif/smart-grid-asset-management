# Repository instructions for agentic coding tools

This file applies to the whole repository. It is the canonical source of
architecture conventions, commands and "definition of done" for any AI coding
agent (or human) working here.

## Architecture

- `backend/` — Go API.
  - `internal/domain` — pure hierarchy/vocabulary rules, no I/O, no DB.
  - `internal/importer` — CSV parsing + validation, pure functions, no DB
    access (takes a snapshot of existing DB rows as a plain map instead).
  - `internal/repository` — all SQL (pgx). No business rules here beyond what
    the schema itself enforces.
  - `internal/service` — orchestrates one use case (import) across importer +
    repository. This is where transaction/commit decisions live.
  - `internal/api` — HTTP handlers/routing only. Handlers translate
    errors to HTTP status codes; they do not contain business rules.
  - `internal/db` — connection pool + embedded migration runner.
- `frontend/` — React + TypeScript SPA (Vite).
  - `src/api/client.ts` — the only place that calls `fetch`.
  - `src/hooks/` — stateful logic (tree loading, debouncing) kept out of
    components.
  - `src/components/` — presentation, one responsibility per file.
- Keep the hierarchy rules (permitted parent/child types, root type) defined
  once in `backend/internal/domain` and mirrored only where a UI label is
  needed — do not duplicate the *rules* in the frontend.

## Development commands

Backend (from `backend/`):
```
go build ./...
go test ./...
go run ./cmd/server
```

Frontend (from `frontend/`):
```
npm install
npm run dev
npm run test
npm run build
```

Full stack:
```
docker compose up --build
```

## Constraints

- CSV row order must never affect import validity or the resulting rejection
  set — any change to `importer.Validate` must preserve this.
- Do not weaken the deferred FK / permitted-parent trigger in
  `internal/db/migrations` — they are the last line of defense against an
  inconsistent hierarchy.
- No authentication, asset editing/deletion, GIS, telemetry, or RBAC — these
  are explicitly out of scope for this assignment.
- The frontend must only ever call the backend's own API (no direct DB
  access, no calls to third-party services).
- Prefer adding a migration file over altering `0001_init.sql` once it has
  been applied anywhere.

## Definition of done

- `go build ./...` and `go test ./...` pass in `backend/`.
- `npm run build` and `npm run test` pass in `frontend/`.
- New backend behaviour affecting import/hierarchy rules has a corresponding
  test in `internal/domain` or `internal/importer`.
- New API behaviour is reflected in the README's endpoint table if it adds or
  changes a route.
- Any known gap or deferred item is recorded in `KNOWN_LIMITATIONS.md`, not
  left undocumented.
