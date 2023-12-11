CREATE TABLE blocks (
    inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    id BIGSERIAL PRIMARY KEY,
    hash BYTEA NOT NULL,
    prevhash BYTEA NOT NULL,
    merkleroot BYTEA NOT NULL,
    height BIGINT NOT NULL,
    processed_at TIMESTAMPTZ,
    size BIGINT,
    tx_count BIGINT,
    orphanedyn BOOLEAN NOT NULL DEFAULT FALSE,
    merkle_path TEXT DEFAULT '' :: TEXT
);

CREATE UNIQUE INDEX ux_blocks_hash ON blocks (hash);

CREATE UNIQUE INDEX pux_blocks_height ON blocks(height)
WHERE
    orphanedyn = FALSE;
