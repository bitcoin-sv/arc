-- move merkle_path to block_transactions_map, because in case there are
-- competing blocks - there may be multiple merkle paths for one transaction
ALTER TABLE blocktx.transactions DROP COLUMN merkle_path;

ALTER TABLE blocktx.block_transactions_map
ADD COLUMN merkle_path TEXT DEFAULT ('');
