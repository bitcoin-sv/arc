CREATE SCHEMA callbacker;

CREATE TABLE callbacker.callbacks (
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

CREATE UNIQUE INDEX ux_callbacker_callbacks_unique ON callbacker.callbacks (url, token, tx_id, tx_status);
