CREATE UNIQUE INDEX IF NOT EXISTS ux_blocks_inserted_at ON blocks (inserted_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_transactions_inserted_at ON transactions (inserted_at);
CREATE UNIQUE INDEX IF NOT EXISTS ux_block_transactions_map_inserted_at ON block_transactions_map (inserted_at);
