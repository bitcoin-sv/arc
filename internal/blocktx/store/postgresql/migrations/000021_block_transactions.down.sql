
DROP TABLE blocktx.block_transactions;
DROP TABLE blocktx.registered_transactions;

DROP INDEX ix_block_transactions_hash;

-- Todo: Recreate blocktx.block_transactions_map & blocktx.transactions
CREATE TABLE blocktx.block_transactions_map (
    blockid BIGINT NOT NULL,
    txid BIGINT NOT NULL,
    pos BIGINT NOT NULL,
    PRIMARY KEY (blockid, txid)
);
