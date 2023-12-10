CREATE TABLE metamorph.blocks (
    hash BYTEA PRIMARY KEY,
    processed_at TIMESTAMPTZ
);

CREATE INDEX ix_metamorph_blocks_processed_at ON metamorph.blocks (processed_at);
