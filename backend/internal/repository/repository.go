package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sivakumarkam/smart-grid/backend/internal/domain"
	"github.com/sivakumarkam/smart-grid/backend/internal/importer"
)

// ErrNotFound is returned when a requested asset does not exist.
var ErrNotFound = errors.New("asset not found")

const assetColumns = `asset_id,
	asset_type::text,
	asset_name,
	parent_asset_id,
	operational_status::text,
	to_char(commissioned_date, 'YYYY-MM-DD'),
	rating_kva::float8,
	voltage_kv::float8,
	location,
	manufacturer,
	model,
	serial_number`

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Pool() *pgxpool.Pool { return r.pool }

func scanAssets(rows pgx.Rows) ([]domain.Asset, error) {
	defer rows.Close()
	out := []domain.Asset{}
	for rows.Next() {
		var a domain.Asset
		if err := rows.Scan(&a.AssetID, &a.AssetType, &a.AssetName, &a.ParentAssetID,
			&a.OperationalStatus, &a.CommissionedDate, &a.RatingKVA, &a.VoltageKV, &a.Location,
			&a.Manufacturer, &a.Model, &a.SerialNumber); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string) (domain.Asset, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+assetColumns+` FROM assets WHERE asset_id = $1`, id)
	if err != nil {
		return domain.Asset{}, err
	}
	assets, err := scanAssets(rows)
	if err != nil {
		return domain.Asset{}, err
	}
	if len(assets) == 0 {
		return domain.Asset{}, ErrNotFound
	}
	return assets[0], nil
}

func (r *Repository) Exists(ctx context.Context, id string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM assets WHERE asset_id = $1)`, id).Scan(&ok)
	return ok, err
}

// Roots returns the substations, i.e. the top level of the explorer tree.
func (r *Repository) Roots(ctx context.Context) ([]domain.Asset, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+assetColumns+` FROM assets WHERE parent_asset_id IS NULL ORDER BY asset_name, asset_id`)
	if err != nil {
		return nil, err
	}
	return scanAssets(rows)
}

func (r *Repository) Children(ctx context.Context, id string) ([]domain.Asset, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT `+assetColumns+` FROM assets WHERE parent_asset_id = $1
		 ORDER BY asset_type, asset_name, asset_id`, id)
	if err != nil {
		return nil, err
	}
	return scanAssets(rows)
}

// Ancestors walks up the self-referencing relationship, root first.
func (r *Repository) Ancestors(ctx context.Context, id string) ([]domain.Asset, error) {
	rows, err := r.pool.Query(ctx, `
		WITH RECURSIVE chain AS (
			SELECT a.*, 0 AS depth FROM assets a WHERE a.asset_id = $1
			UNION ALL
			SELECT p.*, chain.depth + 1
			FROM assets p JOIN chain ON p.asset_id = chain.parent_asset_id
		)
		SELECT `+assetColumns+` FROM chain WHERE depth > 0 ORDER BY depth DESC`, id)
	if err != nil {
		return nil, err
	}
	return scanAssets(rows)
}

// DescendantCounts rolls up the whole subtree below id, grouped by asset type.
func (r *Repository) DescendantCounts(ctx context.Context, id string) (domain.DescendantCounts, error) {
	var counts domain.DescendantCounts
	rows, err := r.pool.Query(ctx, `
		WITH RECURSIVE subtree AS (
			SELECT asset_id, asset_type FROM assets WHERE parent_asset_id = $1
			UNION ALL
			SELECT c.asset_id, c.asset_type
			FROM assets c JOIN subtree s ON c.parent_asset_id = s.asset_id
		)
		SELECT asset_type::text, count(*) FROM subtree GROUP BY asset_type`, id)
	if err != nil {
		return counts, err
	}
	defer rows.Close()
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return counts, err
		}
		counts.Add(t, n)
	}
	return counts, rows.Err()
}

// Search matches on asset id or name (exact matches rank first) and can be
// narrowed by asset type.
func (r *Repository) Search(ctx context.Context, q string, assetType string, limit int) ([]domain.Asset, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT `+assetColumns+`
		FROM assets
		WHERE (asset_id ILIKE '%' || $1 || '%' OR asset_name ILIKE '%' || $1 || '%')
		  AND ($2::text IS NULL OR asset_type::text = $2::text)
		ORDER BY
			(lower(asset_id) = lower($1) OR lower(asset_name) = lower($1)) DESC,
			(asset_name ILIKE $1 || '%') DESC,
			asset_name, asset_id
		LIMIT $3`, q, nullIfEmpty(assetType), limit)
	if err != nil {
		return nil, err
	}
	return scanAssets(rows)
}

func (r *Repository) CountAssets(ctx context.Context) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM assets`).Scan(&n)
	return n, err
}

// ExistingByIDs loads the assets already stored for the given ids. Used by the
// importer to detect duplicates and to resolve parents that live in the
// database rather than in the uploaded file.
func (r *Repository) ExistingByIDs(ctx context.Context, ids []string) (map[string]importer.ExistingAsset, error) {
	out := map[string]importer.ExistingAsset{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT asset_id, asset_type::text FROM assets WHERE asset_id = ANY($1)`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e importer.ExistingAsset
		if err := rows.Scan(&e.AssetID, &e.AssetType); err != nil {
			return nil, err
		}
		out[e.AssetID] = e
	}
	return out, rows.Err()
}

// InsertAssets writes the accepted rows in one transaction. The self-referencing
// foreign key and the hierarchy trigger are DEFERRABLE INITIALLY DEFERRED, so
// rows may be inserted in any order and are checked as a set at COMMIT.
func (r *Repository) InsertAssets(ctx context.Context, assets []domain.Asset) error {
	if len(assets) == 0 {
		return nil
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows := make([][]any, 0, len(assets))
	for _, a := range assets {
		var commissioned *time.Time
		if a.CommissionedDate != nil {
			t, err := time.Parse("2006-01-02", *a.CommissionedDate)
			if err != nil {
				return fmt.Errorf("asset %s: %w", a.AssetID, err)
			}
			commissioned = &t
		}
		rows = append(rows, []any{
			a.AssetID, a.AssetType, a.AssetName, a.ParentAssetID, a.OperationalStatus,
			commissioned, a.RatingKVA, a.VoltageKV, a.Location, a.Manufacturer, a.Model, a.SerialNumber,
		})
	}
	_, err = tx.CopyFrom(ctx, pgx.Identifier{"assets"},
		[]string{"asset_id", "asset_type", "asset_name", "parent_asset_id",
			"operational_status", "commissioned_date", "rating_kva", "voltage_kv", "location",
			"manufacturer", "model", "serial_number"},
		pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("bulk insert: %w", err)
	}
	return tx.Commit(ctx)
}

// RecordImport persists the audit trail. It runs outside the data transaction
// so the report survives a rolled-back all-or-nothing import.
func (r *Repository) RecordImport(ctx context.Context, filename, mode string,
	total, imported, rejected int, committed bool, rejections []importer.Rejection) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO imports (filename, mode, total_rows, imported_rows, rejected_rows, committed)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		filename, mode, total, imported, rejected, committed).Scan(&id); err != nil {
		return 0, err
	}
	if len(rejections) > 0 {
		batch := &pgx.Batch{}
		for _, rj := range rejections {
			batch.Queue(`INSERT INTO import_rejections (import_id, row_number, asset_id, field, message, raw_row)
				VALUES ($1, $2, $3, $4, $5, $6)`,
				id, rj.RowNumber, nullIfEmpty(rj.AssetID), nullIfEmpty(rj.Field), rj.Message, nullIfEmpty(rj.RawRow))
		}
		if err := tx.SendBatch(ctx, batch).Close(); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit(ctx)
}

func nullIfEmpty(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
