CREATE TABLE blocks
(
    hash         BYTEA PRIMARY KEY,
    processed_at TIMESTAMPTZ,
    inserted_at  TIMESTAMPTZ
);

CREATE INDEX ix_metamorph_blocks_inserted_at ON blocks (inserted_at);
