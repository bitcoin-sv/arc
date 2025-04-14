
CREATE INDEX IF NOT EXISTS ix_block_transactions_block_id ON blocktx.block_transactions(block_id);
CREATE INDEX IF NOT EXISTS ix_block_blocks_id ON blocktx.blocks(id);

ALTER TABLE blocktx.block_transactions ADD COLUMN inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;
