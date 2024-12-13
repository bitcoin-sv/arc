-- When reverting this migration, make sure to start calculating
-- and storing ALL merkle_paths for all transactions in blocks

ALTER TABLE blocktx.block_transactions_map
DROP COLUMN merkle_tree_index;
