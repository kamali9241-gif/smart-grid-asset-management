# AI Development Journal

## Tools and models

- **GitHub Copilot** in agent mode, VS Code, using **Claude Sonnet 5** as the
  underlying model for both the backend implementation and this session's
  frontend/infrastructure/documentation work.

## How the work was decomposed and sequenced

1. **Domain + schema first** — defined the permitted parent/child hierarchy
   and enums in `backend/internal/domain` (pure, no I/O) before touching SQL
   or HTTP, so the rules could be unit tested in isolation and referenced
   from both the importer and the database migration.
2. **PostgreSQL schema** (`0001_init.sql`) — self-referencing `assets` table
   with deferred FK + trigger to enforce the hierarchy at the DB level as a
   safety net, plus `imports`/`import_rejections` for the audit trail.
3. **CSV parsing/validation** (`internal/importer`) — pure, DB-agnostic
   functions (`Parse`, `Validate`) so row-order independence and the full
   rejection matrix could be exercised with plain unit tests, no database
   required.
4. **Repository + service + API** — SQL access, the transactional
   all-or-nothing/partial import decision, and HTTP handlers, wired together
   last once the rules underneath were already tested.
5. **Frontend** — types and API client mirroring the backend's JSON contracts
   first, then a lazily-loaded tree hook (`useAssetTree`), then the
   Import and Explorer pages, then styling and component-level tests.
6. **Infrastructure and docs** — Dockerfiles, `docker-compose.yml`,
   `README.md`, `ARCHITECTURE.md`, `KNOWN_LIMITATIONS.md`, `AGENTS.md`, this
   journal — written last, once the implementation they describe existed and
   had been built/tested.

## Three representative prompts

1. *"Build a web application that accepts the supplied CSV, validates it
   [row order must not determine validity], stores accepted records in
   PostgreSQL, and lets users explore or search the resulting asset
   hierarchy — implement the backend first: domain rules, schema, CSV
   parser/validator with unit tests, then the repository/service/API layer."*
2. *"Scaffold the React + TypeScript frontend: an Import page (file picker,
   mode selector, results table with rejections), an Explorer page with a
   lazily-loaded tree, search, and a details panel showing ancestors,
   grouped children and descendant counts, using `/explorer/:assetId` as a
   stable route so refresh/direct navigation keep the selection."*
3. *"Add Dockerfiles for both services and a docker-compose.yml wiring
   Postgres, the Go backend and an nginx-served frontend build together, then
   write the README, architecture/trade-off note, known limitations and
   AGENTS.md."*

## A suggestion rejected / corrected

Initial attempt used `npm create vite@latest` to scaffold the frontend. The
installed `create-vite@9.2.0` required Node `^20.19 || >=22.12`, but the
machine had Node 21.5.0 installed, and the CLI crashed with a `styleText`
import error from `node:util` before writing any files. Rather than asking
the user to change their global Node version (a heavier, less reversible ask
for what should be a local scaffolding step), the project files (`package.json`,
`tsconfig*.json`, `vite.config.ts`, `index.html`, `src/*`) were hand-authored
directly, matching what the CLI template would have produced, and verified by
running `npm install`, `npm run test` and `npm run build` afterwards.

## How generated code was reviewed and verified

- Backend: `go build ./...` and `go test ./...` run after every meaningful
  change; the existing `domain` and `importer` test suites (assertions on
  row-order independence, cycle detection, duplicate IDs, partial-mode
  branch rejection, etc.) were read in full to confirm they actually
  exercised the documented CSV rules rather than trusting the implementation
  by inspection alone.
- Frontend: component/unit tests added for the API client (success, error
  mapping, multipart upload body, network failure) and shared state
  components; `npm run build` (which runs `tsc -b` first) used as a
  type-correctness gate, not just `vite build`.
- Manual read-through of the SQL migration to confirm the deferred
  constraint trigger genuinely allows children-before-parents insert order
  (the assignment's core "row order must not determine validity"
  requirement) rather than assuming it from the code alone.

## Where the agent did poorly / needed more context

- Guessed a Node/npm scaffolding path that failed outright on the installed
  Node version and had to be re-approached manually (see above).
- Docker Desktop was in Windows-container mode on the dev machine, which
  isn't discoverable from the repository itself; the agent had to detect the
  failure from a `docker compose up` error message and ask the user to
  switch modes rather than silently working around it, since flipping
  Docker Desktop's container runtime is a machine-level, not
  repository-level, action.
- The supplied `grid_assets.csv` has `manufacturer`, `model` and
  `serial_number` columns that an earlier pass of the importer silently
  dropped (they weren't in the recognised column list). This was only
  noticed while writing this journal and cross-checking the importer against
  the actual sample file, not from reading the validator code in isolation —
  a reminder that "does it compile and pass its existing tests" is not the
  same as "does it round-trip the real input file". Fixed with an additive
  migration (`0002_asset_equipment_details.sql`) plus matching changes to
  `domain.Asset`, the importer's column aliases, the repository, and the
  frontend details panel.

## Approximate human working time

Roughly **3–4 hours** of active human direction, prompt-writing and review
across the backend, frontend and infrastructure/documentation passes,
excluding unattended agent execution time (installs, builds, test runs).
