CREATE SCHEMA metamorph;
CREATE TABLE global.Transactions (
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
    locked_by TEXT,
    inserted_at_num INTEGER DEFAULT TO_NUMBER(TO_CHAR((NOW()) AT TIME ZONE 'UTC', 'yyyymmddhh24'), '9999999999') NOT NULL
);

CREATE INDEX ix_metamorph_transactions_locked_by ON global.Transactions (locked_by);
CREATE INDEX ix_metamorph_transactions_inserted_at_num ON global.Transactions (inserted_at_num);
