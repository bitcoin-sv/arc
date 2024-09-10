-- move merkle_path to block_transactions_map, because in case there are
-- competing blocks - there may be multiple merkle paths for one transaction
ALTER TABLE blocktx.block_transactions_map
ADD COLUMN merkle_path TEXT DEFAULT ('');

UPDATE blocktx.block_transactions_map AS btm
SET merkle_path = t.merkle_path
FROM blocktx.transactions t
WHERE btm.txid = t.id;

ALTER TABLE blocktx.transactions DROP COLUMN merkle_path;
