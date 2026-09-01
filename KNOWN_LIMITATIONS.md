# Known Limitations

## Functional

- **No editing/deletion of individual assets** — explicitly out of scope per
  the assignment.
- **No authentication or user accounts** — explicitly out of scope.
- **Search** is substring/ILIKE + trigram-index based, not a full-text search
  engine; ranking is a simple "exact match, then prefix match, then
  alphabetical" ordering, not relevance scoring.
- **Re-uploading a corrected file for previously-rejected rows** works (the
  asset didn't exist before), but there is no way to re-import a corrected
  version of a row that was *already accepted* — updates are not supported.
- **Large files**: the importer buffers the whole parsed file in memory
  before validating; this is fine for the documented dataset size (hundreds
  of rows) but was not tuned for bulk (production-scale) ingestion, which the
  assignment says is out of scope.
- **CSV encoding**: only UTF-8 (with or without BOM) is explicitly handled.
  Other encodings are not detected or transcoded.

## Infrastructure / verification

- The containerized stack (`docker-compose.yml`) was authored and the
  backend/frontend were each built and unit-tested independently, but a
  full `docker compose up` run had not yet been verified end-to-end at the
  time of writing because Docker Desktop was mid-install/being switched to
  Linux container mode on the development machine. Update this note once
  that run has been confirmed.
- No CI pipeline (e.g. GitHub Actions) is included; tests are run locally via
  `go test ./...` and `npm run test`.
- No cloud deployment is included (explicitly a bonus, not required).

## What would be implemented next

1. End-to-end (e.g. Playwright) tests covering the upload → explore → search
   → deep-link flows against the real backend/DB.
2. Backend integration tests against a real (or `testcontainers`) Postgres
   instance for `repository` and `service`, complementing the existing pure
   unit tests in `domain` and `importer`.
3. Pagination for `/api/assets/search` if the dataset were expected to grow
   significantly.
4. A structured request-level audit log correlated with the `imports` table
   (currently only the import audit row + request logs, not linked by ID in
   the log line).
