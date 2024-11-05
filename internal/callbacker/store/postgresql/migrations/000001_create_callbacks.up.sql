CREATE SCHEMA callbacker;

CREATE TABLE callbacker.callbacks (
    id BIGSERIAL PRIMARY KEY,
    url TEXT NOT NULL,
    token TEXT NOT NULL,
    tx_id TEXT NOT NULL,
    tx_status TEXT NOT NULL,
    extra_info TEXT,
    merkle_path TEXT,
    block_hash TEXT,
    block_height BIGINT,
    competing_txs TEXT,
    timestamp TIMESTAMPTZ NOT NULL
);
