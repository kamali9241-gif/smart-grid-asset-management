-- Asset master data for the smart grid explorer.
-- The hierarchy is modelled relationally with a self-referencing foreign key;
-- the FK is DEFERRABLE so a bulk import can insert children before parents
-- inside a single transaction and still be checked at COMMIT.

CREATE EXTENSION IF NOT EXISTS pg_trgm;

DO $$ BEGIN
    CREATE TYPE asset_type AS ENUM (
        'SUBSTATION', 'TRANSFORMER', 'LV_BOARD', 'SWITCHBOARD', 'SWITCHBOARD_PANEL'
    );
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE operational_status AS ENUM ('IN_SERVICE', 'MAINTENANCE', 'OUT_OF_SERVICE');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

CREATE TABLE IF NOT EXISTS assets (
    asset_id           TEXT PRIMARY KEY,
    asset_type         asset_type NOT NULL,
    asset_name         TEXT NOT NULL CHECK (length(btrim(asset_name)) > 0),
    parent_asset_id    TEXT REFERENCES assets (asset_id)
                            ON DELETE RESTRICT
                            DEFERRABLE INITIALLY DEFERRED,
    operational_status operational_status NOT NULL DEFAULT 'IN_SERVICE',
    commissioned_date  DATE,
    rating_kva         NUMERIC(12, 2) CHECK (rating_kva IS NULL OR rating_kva >= 0),
    voltage_kv         NUMERIC(10, 3) CHECK (voltage_kv IS NULL OR voltage_kv >= 0),
    location           TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Substations are roots; everything else must hang off a parent.
    CONSTRAINT assets_root_shape CHECK (
        (asset_type = 'SUBSTATION' AND parent_asset_id IS NULL)
        OR (asset_type <> 'SUBSTATION' AND parent_asset_id IS NOT NULL)
    ),
    CONSTRAINT assets_no_self_parent CHECK (parent_asset_id IS DISTINCT FROM asset_id)
);

CREATE INDEX IF NOT EXISTS assets_parent_idx ON assets (parent_asset_id);
CREATE INDEX IF NOT EXISTS assets_type_idx ON assets (asset_type);
CREATE INDEX IF NOT EXISTS assets_name_trgm_idx ON assets USING gin (asset_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS assets_id_trgm_idx ON assets USING gin (asset_id gin_trgm_ops);

-- Enforce the permitted parent/child type pairs in the database as well as in
-- the application, so an out-of-band write cannot corrupt the tree.
CREATE OR REPLACE FUNCTION assert_permitted_parent() RETURNS TRIGGER AS $$
DECLARE
    parent_type asset_type;
BEGIN
    IF NEW.parent_asset_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT asset_type INTO parent_type FROM assets WHERE asset_id = NEW.parent_asset_id;
    IF parent_type IS NULL THEN
        RAISE EXCEPTION 'parent % does not exist', NEW.parent_asset_id;
    END IF;
    IF NOT (
        (NEW.asset_type IN ('TRANSFORMER', 'LV_BOARD', 'SWITCHBOARD') AND parent_type = 'SUBSTATION')
        OR (NEW.asset_type = 'SWITCHBOARD_PANEL' AND parent_type = 'SWITCHBOARD')
    ) THEN
        RAISE EXCEPTION '% may not be a child of %', NEW.asset_type, parent_type;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS assets_permitted_parent ON assets;
CREATE CONSTRAINT TRIGGER assets_permitted_parent
    AFTER INSERT OR UPDATE ON assets
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION assert_permitted_parent();

-- Audit trail for every upload attempt, including rolled-back ones.
CREATE TABLE IF NOT EXISTS imports (
    id            BIGSERIAL PRIMARY KEY,
    filename      TEXT NOT NULL,
    mode          TEXT NOT NULL,
    total_rows    INTEGER NOT NULL,
    imported_rows INTEGER NOT NULL,
    rejected_rows INTEGER NOT NULL,
    committed     BOOLEAN NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS import_rejections (
    id         BIGSERIAL PRIMARY KEY,
    import_id  BIGINT NOT NULL REFERENCES imports (id) ON DELETE CASCADE,
    row_number INTEGER NOT NULL,
    asset_id   TEXT,
    field      TEXT,
    message    TEXT NOT NULL,
    raw_row    TEXT
);

CREATE INDEX IF NOT EXISTS import_rejections_import_idx ON import_rejections (import_id);
