
DROP INDEX IF EXISTS blocktx.ix_block_transactions_block_id;
DROP INDEX IF EXISTS blocktx.ix_block_blocks_id;

ALTER TABLE blocktx.block_transactions DROP COLUMN inserted_at;
