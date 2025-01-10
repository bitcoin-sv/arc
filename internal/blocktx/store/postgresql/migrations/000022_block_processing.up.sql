ALTER TABLE blocktx.block_processing DROP CONSTRAINT block_processing_pkey;

CREATE INDEX ix_block_processing_inserted_at ON blocktx.block_processing (inserted_at);
