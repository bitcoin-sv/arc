CREATE SCHEMA metamorph;
CREATE TABLE metamorph.transactions (
    hash BYTEA PRIMARY KEY,
    stored_at TIMESTAMPTZ,
    announced_at TIMESTAMPTZ,
    mined_at TIMESTAMPTZ,
    status INTEGER,
    block_height BIGINT,
    block_hash BYTEA,
    callback_url TEXT,
    callback_token TEXT,
    merkle_proof TEXT,
    reject_reason TEXT,
    raw_tx BYTEA,
    locked_by TEXT
);

CREATE INDEX ix_metamorph_transactions_stored_at ON metamorph.transactions (stored_at);
