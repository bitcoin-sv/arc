CREATE TABLE transactions
(
    hash           BYTEA PRIMARY KEY,
    stored_at      TIMESTAMPTZ,
    announced_at   TIMESTAMPTZ,
    mined_at       TIMESTAMPTZ,
    status         INTEGER,
    block_height   BIGINT,
    block_hash     BYTEA,
    callback_url   TEXT,
    callback_token TEXT,
    merkle_proof   TEXT,
    reject_reason  TEXT,
    raw_tx         BYTEA,
    locked_by      TEXT,
    inserted_at    TIMESTAMPTZ
);

CREATE INDEX ix_metamorph_transactions_locked_by ON transactions (locked_by);
CREATE INDEX ix_metamorph_transactions_inserted_at_num ON transactions (inserted_at);
