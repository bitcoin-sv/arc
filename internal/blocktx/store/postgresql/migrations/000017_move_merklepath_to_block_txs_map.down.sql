ALTER TABLE blocktx.transactions
ADD COLUMN merkle_path TEXT DEFAULT ('');

UPDATE blocktx.transactions AS t
SET merkle_path = btm.merkle_path
FROM blocktx.block_transactions_map btm
WHERE t.id = btm.txid;

ALTER TABLE blocktx.block_transactions_map DROP COLUMN merkle_path;
