ALTER TABLE blocktx.block_transactions_map
ADD COLUMN merkle_tree_index BIGINT DEFAULT -1; -- this means no merkle_tree_index
