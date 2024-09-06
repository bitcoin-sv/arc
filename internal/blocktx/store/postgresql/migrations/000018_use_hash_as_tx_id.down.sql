ALTER TABLE blocktx.transactions
DROP CONSTRAINT transactions_pkey;

ALTER TABLE blocktx.transactions
ADD COLUMN id;

CREATE UNIQUE INDEX ux_transactions_hash ON blocktx.transactions (hash);

ALTER TABLE blocktx.transactions
ADD PRIMARY KEY (id);


-- revert the updates on block_transactions_map
ALTER TABLE blocktx.block_transactions_map
DROP CONSTRAINT block_transactions_map_pkey;

ALTER TABLE blocktx.block_transactions_map
DROP COLUMN tx_hash;

ALTER TABLE blocktx.block_transactions_map
ADD COLUMN txid BIGINT NOT NULL;

ALTER TABLE blocktx.block_transactions_map
ADD PRIMARY KEY (blockid, txid);
