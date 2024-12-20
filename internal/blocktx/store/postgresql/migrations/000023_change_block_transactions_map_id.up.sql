ALTER TABLE blocktx.block_transactions_map
ADD COLUMN txhash BYTEA NOT NULL;

UPDATE blocktx.block_transactions_map AS btm
SET txhash = t.hash
FROM blocktx.transactions t
WHERE btm.txid = t.id;

ALTER TABLE blocktx.block_transactions_map DROP CONSTRAINT block_transactions_map_pkey;
ALTER TABLE blocktx.block_transactions_map DROP COLUMN txid;

ALTER TABLE blocktx.block_transactions_map ADD PRIMARY KEY (blockid, txhash);
