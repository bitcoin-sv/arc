ALTER TABLE blocktx.block_processing ADD PRIMARY KEY (block_hash);

DROP INDEX ix_block_processing_inserted_at;
