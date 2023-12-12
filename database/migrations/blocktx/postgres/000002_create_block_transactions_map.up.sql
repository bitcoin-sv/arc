CREATE TABLE block_transactions_map (
    blockid BIGINT NOT NULL,
    txid BIGINT NOT NULL,
    pos BIGINT NOT NULL,
    PRIMARY KEY (blockid, txid)
);
