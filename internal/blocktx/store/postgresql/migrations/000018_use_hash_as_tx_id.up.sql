ALTER TABLE blocktx.transactions
DROP CONSTRAINT transactions_pkey;

ALTER TABLE blocktx.transactions
DROP COLUMN id;

-- we don't need that index because we're setting a primary key on that field
ALTER TABLE blocktx.transactions
DROP INDEX ux_transactions_hash;

ALTER TABLE blocktx.transactions
ADD PRIMARY KEY (hash);


-- update the block_transactions_map table
ALTER TABLE blocktx.block_transactions_map
DROP CONSTRAINT block_transactions_map_pkey;

ALTER TABLE blocktx.block_transactions_map
DROP COLUMN txid;

ALTER TABLE blocktx.block_transactions_map
ADD COLUMN tx_hash BYTEA NOT NULL;

ALTER TABLE blocktx.block_transactions_map
ADD PRIMARY KEY (blockid, tx_hash);

