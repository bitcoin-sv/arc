DROP INDEX ix_blocks_inserted_at;

ALTER TABLE blocks
ADD COLUMN inserted_at_num INTEGER DEFAULT TO_NUMBER(TO_CHAR((NOW()) AT TIME ZONE 'UTC', 'yyyymmddhh24'), '9999999999') NOT NULL;

CREATE INDEX ix_blocks_inserted_at ON blocks (inserted_at_num);
