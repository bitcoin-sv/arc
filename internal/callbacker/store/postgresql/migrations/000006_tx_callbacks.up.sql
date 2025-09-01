CREATE TABLE callbacker.transaction_callbacks
(
    id            BIGSERIAL          NOT NULL,
    url           TEXT               NOT NULL,
    "token"       TEXT               NOT NULL,
    tx_id         TEXT               NOT NULL,
    tx_status     TEXT               NOT NULL,
    extra_info    TEXT               NULL,
    merkle_path   TEXT               NULL,
    block_hash    TEXT               NULL,
    block_height  int8               NULL,
    competing_txs TEXT               NULL,
    "timestamp"   timestamptz        NOT NULL,
    allow_batch   bool DEFAULT false NULL,
    sent_at       timestamptz        NULL,
    pending       timestamptz        NULL,
    CONSTRAINT transaction_callbacks_pkey PRIMARY KEY (id),
    CONSTRAINT unique_url_tx_id_status_block_hash UNIQUE (url, tx_id, tx_status, block_hash)
);
CREATE INDEX ix_callbacks_sent_at ON callbacker.transaction_callbacks USING btree (sent_at);
