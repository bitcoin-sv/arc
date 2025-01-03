CREATE TABLE blocktx.block_transactions (
    block_id BIGINT,
    hash BYTEA NOT NULL,
    merkle_tree_index BIGINT DEFAULT -1, -- this means no merkle_tree_index
    inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (hash, block_id),
    CONSTRAINT fk_block
        FOREIGN KEY(block_id)
        REFERENCES blocktx.blocks(id)
        ON DELETE CASCADE
);

CREATE INDEX ix_block_transactions_hash ON blocktx.block_transactions (hash);

CREATE TABLE blocktx.registered_transactions (
    hash BYTEA PRIMARY KEY,
    inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);


-- Todo: migrate txs from block transactions map & transactions to new tables


DROP TABLE blocktx.block_transactions_map;
DROP TABLE blocktx.transactions;
