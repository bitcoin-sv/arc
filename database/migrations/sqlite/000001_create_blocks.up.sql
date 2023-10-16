CREATE TABLE blocks (
                        inserted_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
                        id INTEGER NOT NULL,
                        hash BLOB NOT NULL,
                        prevhash BLOB NOT NULL,
                        merkleroot BLOB NOT NULL,
                        height INTEGER NOT NULL,
                        processed_at DATETIME,
                        size INTEGER,
                        tx_count INTEGER,
                        orphanedyn BOOLEAN DEFAULT 0 NOT NULL,
                        merkle_path TEXT DEFAULT ''
);

CREATE INDEX ix_blocks_hash ON blocks (hash);
CREATE UNIQUE INDEX pux_blocks_height ON blocks(height) WHERE orphanedyn = FALSE;