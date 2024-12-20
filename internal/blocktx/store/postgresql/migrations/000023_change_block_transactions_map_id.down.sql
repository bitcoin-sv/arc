ALTER TABLE blocktx.block_transactions_map
ADD COLUMN txid INTEGER;

ALTER TABLE blocktx.block_transactions_map
DROP CONSTRAINT block_transactions_map_pkey;

ALTER TABLE blocktx.block_transactions_map
ADD PRIMARY KEY (blockid, txid);

UPDATE blocktx.block_transactions_map AS btm
SET txid = t.id
FROM blocktx.transactions t
WHERE btm.txhash = t.hash;

ALTER TABLE blocktx.block_transactions_map
DROP COLUMN txhash;
