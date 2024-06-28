DROP INDEX ix_blocks_inserted_at;
ALTER TABLE blocks DROP COLUMN inserted_at_num;

CREATE INDEX ix_blocks_inserted_at ON blocks (inserted_at);
