
CREATE INDEX ix_block_transactions_hash_merkle_tree ON blocktx.block_transactions(hash, merkle_tree_index);
