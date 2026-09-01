# Smart Grid Asset Management

A web application for importing, validating and exploring a hierarchy of
substations and grid equipment supplied as CSV extracts.

- **Frontend:** React + TypeScript (Vite)
- **Backend:** Go (chi router, pgx driver)
- **Database:** PostgreSQL 16

See [ARCHITECTURE.md](ARCHITECTURE.md) for design decisions and trade-offs,
[KNOWN_LIMITATIONS.md](KNOWN_LIMITATIONS.md) for what is intentionally not
handled yet, and [AI_JOURNAL.md](AI_JOURNAL.md) / [AGENTS.md](AGENTS.md) for
the agentic-development record required by the assignment.

## Prerequisites

| Tool | Version used | Required for |
|---|---|---|
| Docker Desktop + Compose v2 | 25.x / v2.27 | Running the full stack in containers (recommended) |
| Go | 1.24+ | Running/testing the backend outside Docker |
| Node.js | 20.19+ or 22.12+ (project developed against 21.x) | Running/testing the frontend outside Docker |
| PostgreSQL | 16 | Running the backend outside Docker |

## Quick start (Docker Compose)

```powershell
docker compose up --build
```

This starts three containers:

- `postgres` on `localhost:5432` (user/password/db: `grid`/`grid`/`grid_assets`)
- `backend` on `http://localhost:8080` (applies migrations automatically on boot)
- `frontend` on `http://localhost:8081` (nginx serving the built SPA and proxying `/api` to the backend)

Open `http://localhost:8081`, go to **Import**, and upload `grid_assets.csv`
from the repository root. Then switch to **Explorer** to browse the tree and
search.

To stop and remove the containers (keeping the database volume):

```powershell
docker compose down
```

To also drop all imported data:

```powershell
docker compose down -v
```

## Running without Docker

### Database

Start a local PostgreSQL 16 instance and create a database, e.g.:

```sql
CREATE ROLE grid WITH LOGIN PASSWORD 'grid';
CREATE DATABASE grid_assets OWNER grid;
```

### Backend

```powershell
cd backend
copy .env.example .env   # then edit DATABASE_URL if needed
go run ./cmd/server
```

Migrations in `internal/db/migrations` are embedded in the binary and applied
automatically on startup (tracked in a `schema_migrations` table, so re-runs
are idempotent).

Run backend tests:

```powershell
cd backend
go test ./...
```

### Frontend

```powershell
cd frontend
npm install
npm run dev
```

The Vite dev server proxies `/api` to `http://localhost:8080` (override with
`VITE_API_PROXY_TARGET`). Open `http://localhost:5173`.

Run frontend tests:

```powershell
cd frontend
npm run test
```

Build a production bundle (type-checks first):

```powershell
cd frontend
npm run build
```

## Importing data

`grid_assets.csv` at the repository root is the supplied dataset (220 assets
across 13 substations). Upload it via the **Import** page, or with curl:

```powershell
curl -F "file=@grid_assets.csv" -F "mode=all_or_nothing" http://localhost:8080/api/imports
```

Supported `mode` values:

- `all_or_nothing` (default): nothing is written unless every row is valid.
- `partial`: valid rows are committed; rows that depend on a rejected
  ancestor are rejected too, so the stored tree is always complete from a
  substation root down.

The response reports total/imported/rejected row counts and, per rejected
row, the original row number, asset ID (if available) and a human-readable
reason.

## API overview

Full request/response schemas, status codes and examples are in
[API.md](API.md). Quick summary:

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/health` | Liveness + DB connectivity + asset count |
| GET | `/api/meta` | Supported asset types, statuses, import modes |
| POST | `/api/imports` | Upload and validate a CSV (`file`, `mode` form fields) |
| GET | `/api/assets/roots` | Substations (tree roots) |
| GET | `/api/assets/search?q=&type=&limit=` | Search by id/name, optional type filter |
| GET | `/api/assets/{assetId}` | Asset detail: ancestors, children grouped by type, descendant counts |
| GET | `/api/assets/{assetId}/children` | Immediate children |
| GET | `/api/assets/{assetId}/ancestors` | Root-to-parent ancestor chain |

## Repository layout

```
backend/            Go API, domain rules, CSV importer, PostgreSQL access, migrations
frontend/           React + TypeScript SPA (import workflow, asset explorer, search)
grid_assets.csv     Supplied sample dataset
docker-compose.yml  Local containerized stack (postgres + backend + frontend)
```
