# API Reference

Base URL: `http://localhost:8080/api` (direct) or `http://localhost:8081/api`
(through the nginx-fronted frontend container, same routes).

All responses are `application/json`. All errors share one shape:

```json
{ "error": { "code": "asset_not_found", "message": "no asset with id SUB-9" } }
```

| HTTP status | Meaning |
|---|---|
| 400 | Malformed request (bad CSV, missing file, invalid query param) |
| 404 | Asset not found |
| 413 | Uploaded file exceeds `MAX_UPLOAD_MB` |
| 415 | Uploaded file is not `.csv` |
| 422 | Import was validated but not committed (see `POST /imports`) |
| 500 | Unexpected server error |
| 503 | `/health` only: database unreachable |

---

## `GET /health`

Liveness probe. Returns current asset count as a sanity check.

**200 OK**
```json
{ "status": "ok", "assetCount": 205 }
```

---

## `GET /meta`

Supported enums, so the frontend never hard-codes domain rules.

**200 OK**
```json
{
  "assetTypes": ["SUBSTATION", "TRANSFORMER", "LV_BOARD", "SWITCHBOARD", "SWITCHBOARD_PANEL"],
  "operationalStatuses": ["IN_SERVICE", "MAINTENANCE", "OUT_OF_SERVICE"],
  "importModes": ["all_or_nothing", "partial"]
}
```

---

## `POST /imports`

Uploads and validates a CSV file. `multipart/form-data` with fields:

| Field | Required | Values |
|---|---|---|
| `file` | yes | a `.csv` file |
| `mode` | no | `all_or_nothing` (default) or `partial` |

**201 Created** — every row was valid, or `mode=partial` committed the valid subset:
```json
{
  "importId": 12,
  "filename": "grid_assets.csv",
  "mode": "partial",
  "totalRows": 222,
  "importedRows": 205,
  "rejectedRows": 17,
  "committed": true,
  "message": "Valid rows were committed; rejected rows were not written.",
  "rejections": [
    {
      "rowNumber": 91,
      "assetId": "PNL-M",
      "field": "parent_asset_id",
      "message": "parent \"SWB-404\" was not found in the file or in the database",
      "rawRow": "PNL-M,SWB-404,SWITCHBOARD_PANEL,..."
    }
  ]
}
```

**422 Unprocessable Entity** — `mode=all_or_nothing` (default) and at least one
row was rejected: the transaction was rolled back, `committed: false`,
`importedRows: 0`, but the full `rejections` list is still returned so the
caller can show what would have happened. **Callers must treat 422 on this
endpoint as a normal response to parse, not a transport error** — the body is
a complete `ImportReport`, same shape as 201.

**400 Bad Request** — malformed CSV (missing required columns, unreadable
file, duplicate header):
```json
{ "error": { "code": "invalid_csv", "message": "missing required column(s): asset_type" } }
```

---

## `GET /assets/roots`

Substations only (tree roots), ordered by name.

**200 OK**
```json
{ "assets": [ { "assetId": "SS-001", "assetType": "SUBSTATION", "assetName": "Aurora Substation (North)", "parentAssetId": null, "operationalStatus": "IN_SERVICE", "commissionedDate": "2009-02-15", "ratingKva": null, "voltageKv": 22, "location": null, "manufacturer": null, "model": null, "serialNumber": "SUB-7001" } ] }
```

---

## `GET /assets/search?q=&type=&limit=`

| Query param | Required | Notes |
|---|---|---|
| `q` | yes | Matched against `asset_id` and `asset_name`, case-insensitive substring (`ILIKE '%q%'`) |
| `type` | no | One of the `assetTypes` from `/meta`; 400 if invalid |
| `limit` | no | Default 50, max 200 |

Results are ordered: exact match first, then prefix match, then
alphabetically — not full relevance ranking (see `KNOWN_LIMITATIONS.md`).

**200 OK**
```json
{ "query": "panel", "count": 2, "assets": [ { "assetId": "PNL-001-1-02", "assetType": "SWITCHBOARD_PANEL", "assetName": "Aurora Panel A2", "...": "..." } ] }
```

**400 Bad Request** — missing `q`, or unsupported `type`.

---

## `GET /assets/{assetId}`

The single call the Explorer's details panel needs.

**200 OK**
```json
{
  "asset": { "assetId": "PNL-004-1-02", "assetType": "SWITCHBOARD_PANEL", "assetName": "Driftwood Panel A2", "parentAssetId": "SWB-004-1", "...": "..." },
  "ancestors": [
    { "assetId": "SS-004", "assetType": "SUBSTATION", "assetName": "Driftwood Substation (East)" },
    { "assetId": "SWB-004-1", "assetType": "SWITCHBOARD", "assetName": "Driftwood 22kV Switchboard A" }
  ],
  "childrenByType": [
    { "assetType": "SWITCHBOARD_PANEL", "count": 4, "assets": [ "..." ] }
  ],
  "childCount": 4,
  "descendantCounts": { "SUBSTATION": 0, "TRANSFORMER": 0, "LV_BOARD": 0, "SWITCHBOARD": 0, "SWITCHBOARD_PANEL": 0, "total": 0 }
}
```
- `ancestors` is root-first (substation, then any intermediate switchboard).
- `descendantCounts` is only meaningful (non-zero) for a `SUBSTATION`; leaf
  assets return all-zero counts.

**404 Not Found**
```json
{ "error": { "code": "asset_not_found", "message": "no asset with id PNL-999" } }
```

---

## `GET /assets/{assetId}/children`

Immediate children only (one level), ordered by type then name.

**200 OK** → `{ "assets": [ ... ] }`
**404 Not Found** if `assetId` doesn't exist.

---

## `GET /assets/{assetId}/ancestors`

Root-to-parent chain (excludes `assetId` itself), root first.

**200 OK** → `{ "assets": [ ... ] }`
**404 Not Found** if `assetId` doesn't exist.

---

## Field reference (`Asset`)

| Field | Type | Notes |
|---|---|---|
| `assetId` | string | Primary key from the CSV |
| `assetType` | enum | See `/meta` |
| `assetName` | string | |
| `parentAssetId` | string \| null | Null only for `SUBSTATION` |
| `operationalStatus` | enum | Defaults to `IN_SERVICE` if blank in the CSV |
| `commissionedDate` | string (`YYYY-MM-DD`) \| null | |
| `ratingKva` | number \| null | Never negative |
| `voltageKv` | number \| null | |
| `location`, `manufacturer`, `model`, `serialNumber` | string \| null | Optional equipment metadata carried through from the CSV |
