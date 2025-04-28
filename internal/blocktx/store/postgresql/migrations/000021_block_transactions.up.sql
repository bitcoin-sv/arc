CREATE TABLE IF NOT EXISTS blocktx.block_transactions (
    block_id BIGINT,
    hash BYTEA NOT NULL,
    merkle_tree_index BIGINT DEFAULT -1, -- this means no merkle_tree_index
    PRIMARY KEY (hash, block_id),
    CONSTRAINT fk_block
        FOREIGN KEY(block_id)
        REFERENCES blocktx.blocks(id)
        ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS blocktx.registered_transactions (
    hash BYTEA PRIMARY KEY,
    inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS ix_registered_transactions_inserted_at ON blocktx.registered_transactions USING btree (inserted_at);

INSERT INTO blocktx.registered_transactions
SELECT t.hash AS hash FROM blocktx.transactions t WHERE t.is_registered;

DROP INDEX blocktx.ix_block_transactions_map_inserted_at;
DROP TABLE blocktx.block_transactions_map;
DROP INDEX blocktx.ix_transactions_inserted_at;
DROP INDEX blocktx.ux_transactions_hash;
DROP TABLE blocktx.transactions;
