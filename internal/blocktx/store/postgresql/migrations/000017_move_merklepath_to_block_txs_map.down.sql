ALTER TABLE blocktx.block_transactions_map DROP COLUMN merkle_path;

ALTER TABLE blocktx.transactions
ADD COLUMN merkle_path TEXT DEFAULT ('');
