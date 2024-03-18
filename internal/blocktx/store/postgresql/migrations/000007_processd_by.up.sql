
CREATE TABLE block_processing (
        block_hash BYTEA PRIMARY KEY,
        processed_by TEXT DEFAULT '' NOT NULL,
        inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
