-- Remove unused fields
ALTER TABLE blocktx.block_transactions_map DROP COLUMN pos;
ALTER TABLE blocktx.transactions DROP COLUMN source;
