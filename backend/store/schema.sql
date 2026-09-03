-- Entity Matcher Schema
-- This schema is idempotent and self-applies on application startup.

-- Configuration table: single row holds the current matching configuration
CREATE TABLE IF NOT EXISTS config (
    id INTEGER PRIMARY KEY DEFAULT 1,
    config JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT only_one_row CHECK (id = 1)
);

-- Connector settings: single row holds the source/destination data source
-- connection settings the UI shows. Passwords are deliberately NOT stored.
CREATE TABLE IF NOT EXISTS connector_settings (
    id INTEGER PRIMARY KEY DEFAULT 1,
    settings JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT connector_settings_only_one_row CHECK (id = 1)
);

-- Match jobs: high-level summary of each batch matching job
CREATE TABLE IF NOT EXISTS match_jobs (
    batch_id VARCHAR(255) PRIMARY KEY,
    status VARCHAR(50) NOT NULL,
    total_sources INTEGER NOT NULL DEFAULT 0,
    total_destinations INTEGER NOT NULL DEFAULT 0,
    auto_matched INTEGER NOT NULL DEFAULT 0,
    review_needed INTEGER NOT NULL DEFAULT 0,
    no_match_count INTEGER NOT NULL DEFAULT 0,
    total_candidate_pairs INTEGER NOT NULL DEFAULT 0,
    elapsed_ms BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    config JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Match results: individual match candidates between source and destination records
-- PK is composite (batch_id, id) to prevent cross-batch ID collisions
CREATE TABLE IF NOT EXISTS match_results (
    batch_id VARCHAR(255) NOT NULL REFERENCES match_jobs(batch_id) ON DELETE CASCADE,
    id VARCHAR(255) NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    destination_id VARCHAR(255) NOT NULL,
    confidence_score NUMERIC NOT NULL,
    name_score NUMERIC NOT NULL DEFAULT 0,
    date_score NUMERIC NOT NULL DEFAULT 0,
    match_status VARCHAR(50) NOT NULL,
    rank INTEGER NOT NULL DEFAULT 1,
    score_margin NUMERIC NOT NULL DEFAULT 0,
    decision_note TEXT,
    match_reasons JSONB NOT NULL DEFAULT '[]',
    source_snapshot JSONB NOT NULL DEFAULT '{}',
    destination_snapshot JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (batch_id, id)
);

-- Migrate old schema: if match_results has single-column PK, drop and recreate
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE table_name = 'match_results' AND constraint_type = 'PRIMARY KEY'
    ) THEN
        -- Check if the old PK is just on id (single column)
        IF EXISTS (
            SELECT 1 FROM information_schema.key_column_usage
            WHERE table_name = 'match_results' AND constraint_name LIKE 'match_results_pkey'
            AND ordinal_position = 1
            AND column_name = 'id'
        ) AND NOT EXISTS (
            SELECT 1 FROM information_schema.key_column_usage
            WHERE table_name = 'match_results' AND constraint_name LIKE 'match_results_pkey'
            AND ordinal_position = 2
        ) THEN
            -- Old schema detected, migrate
            ALTER TABLE match_results DROP CONSTRAINT match_results_pkey;
            ALTER TABLE match_results ADD PRIMARY KEY (batch_id, id);
        END IF;
    END IF;
END$$;

-- Indexes on frequently-filtered columns
CREATE INDEX IF NOT EXISTS idx_match_results_batch_status
    ON match_results(batch_id, match_status);
CREATE INDEX IF NOT EXISTS idx_match_results_batch_source
    ON match_results(batch_id, source_id);

-- Paging indexes. The default review-queue order is (created_at, id) and the
-- most common user-chosen order is confidence descending; without these,
-- every LIMIT/OFFSET page sorts the whole batch before slicing it.
CREATE INDEX IF NOT EXISTS idx_match_results_batch_created
    ON match_results(batch_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_match_results_batch_confidence
    ON match_results(batch_id, confidence_score DESC, id);

-- Match audit logs: compliance-critical, append-only record of all match decisions
-- Application role should NOT hold TRUNCATE privilege on this table.
CREATE TABLE IF NOT EXISTS match_audit_logs (
    id VARCHAR(255) PRIMARY KEY,
    batch_id VARCHAR(255) NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    destination_id VARCHAR(255) NOT NULL,
    user_id VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    previous_status VARCHAR(50),
    new_status VARCHAR(50),
    confidence_score NUMERIC,
    review_comments TEXT,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Indexes on audit log query patterns
CREATE INDEX IF NOT EXISTS idx_audit_batch
    ON match_audit_logs(batch_id);
CREATE INDEX IF NOT EXISTS idx_audit_user
    ON match_audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_timestamp_desc
    ON match_audit_logs(timestamp DESC);

-- Prevent UPDATE on audit logs: immutability guarantee
CREATE OR REPLACE FUNCTION prevent_audit_update() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'match_audit_logs is append-only: UPDATE is not permitted';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_prevent_update ON match_audit_logs;
CREATE TRIGGER audit_prevent_update
    BEFORE UPDATE ON match_audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_update();

-- Prevent DELETE on audit logs: immutability guarantee
CREATE OR REPLACE FUNCTION prevent_audit_delete() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'match_audit_logs is append-only: DELETE is not permitted';
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS audit_prevent_delete ON match_audit_logs;
CREATE TRIGGER audit_prevent_delete
    BEFORE DELETE ON match_audit_logs
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_delete();

-- Match sources: uploaded source records for a batch, persisted so a match run can be
-- re-executed (or retried after a crash) without re-uploading the file.
-- PK is composite (batch_id, id) to prevent cross-batch ID collisions, mirroring match_results.
-- NOTE: no column for the normalized name (matcher.CleanName). It is derived from
-- customer_name_raw via matcher.Normalize, which is deterministic, so storing it would let it
-- go stale against a future normalizer change; recomputing on read means existing rows benefit
-- from normalizer fixes automatically instead of requiring a backfill migration.
CREATE TABLE IF NOT EXISTS match_sources (
    batch_id VARCHAR(255) NOT NULL REFERENCES match_jobs(batch_id) ON DELETE CASCADE,
    id VARCHAR(255) NOT NULL,
    reference_id VARCHAR(255) NOT NULL,
    customer_name_raw VARCHAR(255) NOT NULL,
    transaction_date TIMESTAMPTZ NOT NULL,
    transaction_type VARCHAR(50) NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY (batch_id, id)
);

CREATE INDEX IF NOT EXISTS idx_match_sources_batch_id
    ON match_sources(batch_id);

-- Match destinations: uploaded destination records for a batch. See match_sources above for
-- why there is no stored normalized-name column.
CREATE TABLE IF NOT EXISTS match_destinations (
    batch_id VARCHAR(255) NOT NULL REFERENCES match_jobs(batch_id) ON DELETE CASCADE,
    id VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    customer_name_raw VARCHAR(255) NOT NULL,
    transaction_date TIMESTAMPTZ NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}',
    PRIMARY KEY (batch_id, id)
);

CREATE INDEX IF NOT EXISTS idx_match_destinations_batch_id
    ON match_destinations(batch_id);

-- Calibration models: fitted score-to-probability calibrators (Platt/Isotonic/Identity), with
-- the metrics they were evaluated at fit time. Append-only for model_json/metrics/observation
-- counts — a re-fit always inserts a new row. The `active` column is the one field callers are
-- expected to update (via UPDATE ... SET active = false on the previous holder, then INSERT the
-- new active row) when promoting a newly-fitted model, so at most one row is active at a time.
CREATE TABLE IF NOT EXISTS calibration_models (
    id VARCHAR(255) PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    fitted_by VARCHAR(255) NOT NULL DEFAULT '',
    batch_id VARCHAR(255) NOT NULL DEFAULT '',
    observation_count INTEGER NOT NULL DEFAULT 0,
    positive_count INTEGER NOT NULL DEFAULT 0,
    brier_score NUMERIC NOT NULL DEFAULT 0,
    ece_score NUMERIC NOT NULL DEFAULT 0,
    model_json JSONB NOT NULL DEFAULT '{}',
    active BOOLEAN NOT NULL DEFAULT false
);

-- Only one model should be active at a time; this partial unique index enforces it at the
-- database level as a defense-in-depth check alongside the application-level deactivate-then-
-- insert logic.
CREATE UNIQUE INDEX IF NOT EXISTS idx_calibration_models_one_active
    ON calibration_models ((active)) WHERE active = true;

CREATE INDEX IF NOT EXISTS idx_calibration_models_created_at_desc
    ON calibration_models(created_at DESC);
