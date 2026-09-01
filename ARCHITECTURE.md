# Architecture & Trade-offs

## Overview

```mermaid
flowchart LR
    Browser -->|HTTP| Frontend[React SPA\n(nginx in prod / Vite dev server)]
    Frontend -->|/api proxy| Backend[Go API\n(chi router)]
    Backend --> Postgres[(PostgreSQL)]
```

- **Frontend** talks only to the backend's HTTP API (no direct DB access),
  satisfying the "browser-based frontend communicates only with your backend"
  requirement.
- **Backend** is a single Go binary: `domain` (pure hierarchy rules), `importer`
  (CSV parsing + validation, no I/O), `repository` (pgx SQL), `service`
  (orchestrates import: parse → validate → persist → audit), `api` (HTTP
  handlers/routing). Each layer is unit-testable independently of the others.
- **Database** is PostgreSQL with a self-referencing `assets` table.

## Hierarchy model

`assets.parent_asset_id` is a nullable self-referencing foreign key. Rather
than trust the application alone, the schema enforces the domain rules
directly:

- `assets_root_shape` CHECK: a `SUBSTATION` has no parent; everything else
  must have one.
- `assets_no_self_parent` CHECK: an asset can't be its own parent.
- `assert_permitted_parent()` trigger: enforces the exact permitted
  parent/child type pairs (e.g. a panel may only belong to a switchboard).
- The FK and the trigger are both `DEFERRABLE INITIALLY DEFERRED`, so a bulk
  import can insert rows in **any order** (children before parents) and the
  constraints are only checked at `COMMIT`. This directly satisfies "CSV row
  order must not determine validity."

Cycle detection and "does this ultimately resolve to a substation root" are
checked in the application layer (`importer.Validate`) before any row reaches
the database, using a DFS over the combined graph of file rows + existing DB
rows. The DB constraints are then a safety net against any future write path
that bypasses the importer.

Indexes: `assets_parent_idx` (child lookups / children endpoint),
`assets_type_idx` (grouping/filtering), and `pg_trgm` GIN indexes on
`asset_id`/`asset_name` for fast partial/ILIKE search.

Ancestor and descendant-count queries use recursive CTEs (`WITH RECURSIVE`)
walking the self-referencing FK — no separate closure table, which keeps
writes simple at the dataset sizes described in the assignment (hundreds of
rows, not millions).

## CSV validation strategy

Validation runs in three stages purely in memory (`importer.Validate`),
independent of the database:

1. **Field-level rules** — required fields, enum membership, numeric/date
   parsing, non-negativity, self-parent check.
2. **Uniqueness** — duplicate `asset_id` within the file, and collision with
   an existing DB row.
3. **Hierarchy resolution** — a DFS resolves each candidate's parent chain
   (from the file or the database) checking permitted type pairs, cycles, and
   that every chain terminates at a substation. A parent already in the
   database is trusted to be acyclic (the DB enforces that), so resolution
   can stop there.

This design makes row order irrelevant by construction: a child is validated
by walking to its parent wherever that parent appears, not by a single linear
pass.

## Import transaction choice

**Both modes are implemented, selectable per upload** (`mode` form field,
default `all_or_nothing`):

- `all_or_nothing`: the accepted rows are only written if there were zero
  rejections. This is the default because a partially-imported substation
  tree could otherwise look "complete" in the explorer while actually missing
  branches, which is misleading for an asset register.
- `partial`: valid rows are committed even if others are rejected. Any row
  that depends (directly or transitively) on a rejected row is rejected too,
  so the tree that lands in the database is always a complete, consistent
  subtree rooted at a substation — never a dangling branch.

Persistence itself is one transaction using `COPY FROM` (`pgx.CopyFrom`) for
throughput, deferred constraints for order-independence, and a rollback on
any error. The import's audit row (`imports` / `import_rejections` tables) is
written in a **separate** transaction so the rejection report survives even
when the data transaction rolls back — this is what lets the UI show "what
would have happened" for an all-or-nothing failure.

## API design

Endpoints are resource-oriented and map directly to explorer needs (roots,
one asset + its context, children, ancestors, search) rather than a generic
CRUD surface, since editing/deleting is explicitly out of scope. Errors are a
single consistent JSON shape (`{"error": {"code", "message"}}`) so the
frontend has one error-handling code path.

## Frontend design

- Route `/explorer/:assetId` is the single source of truth for the selected
  asset — a refresh or deep link re-derives the same view by re-fetching the
  asset and its ancestors, satisfying the "refresh retains selection"
  requirement without any client-side persistence hacks.
- The tree is lazily loaded per node (`GET /children`) rather than fetched
  whole, so opening the explorer doesn't require pulling all 220 assets up
  front; `revealAsset` walks the ancestor chain returned by `GET /ancestors`
  to expand exactly the nodes needed to show a selected/searched asset.
- Search is debounced (250 ms) and cancels stale in-flight responses by
  request-id comparison, to avoid results flickering to an older query.
- Components are split by responsibility (tree node, tree view, search box,
  details panel, state views) and are presentation-only where possible, with
  data-fetching concentrated in `api/client.ts` and the `useAssetTree` hook.

## Trade-offs / things deliberately not done

- No closure table or materialized path for the hierarchy — recursive CTEs
  are simple and fast enough at this scale, but would need revisiting for a
  much deeper/larger tree.
- No optimistic UI updates or client-side caching beyond the in-memory tree
  state; every navigation re-fetches from the API. Simpler and correct, at
  the cost of an extra round trip on repeat visits.
- No pagination on `/assets/search` beyond a `limit` parameter (capped at
  200) — acceptable for a 220-row dataset, would need real pagination at
  production scale.
- No authentication, editing, or deletion, per the assignment's explicit
  scope boundaries.
