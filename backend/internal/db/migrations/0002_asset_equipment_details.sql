-- Captures manufacturer/model/serial_number, present in the supplied CSV but
-- previously dropped by the importer as unrecognised columns.
ALTER TABLE assets
    ADD COLUMN IF NOT EXISTS manufacturer TEXT,
    ADD COLUMN IF NOT EXISTS model        TEXT,
    ADD COLUMN IF NOT EXISTS serial_number TEXT;
