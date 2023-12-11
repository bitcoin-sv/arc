ALTER TABLE blocks
ADD COLUMN inserted_at_num INTEGER DEFAULT TO_NUMBER(TO_CHAR((NOW()) AT TIME ZONE 'UTC', 'yyyymmddhh24'), '9999999999') NOT NULL;

ALTER TABLE transactions
ADD COLUMN inserted_at_num INTEGER DEFAULT TO_NUMBER(TO_CHAR((NOW()) AT TIME ZONE 'UTC', 'yyyymmddhh24'), '9999999999') NOT NULL;

ALTER TABLE block_transactions_map
ADD COLUMN inserted_at_num INTEGER DEFAULT TO_NUMBER(TO_CHAR((NOW()) AT TIME ZONE 'UTC', 'yyyymmddhh24'), '9999999999') NOT NULL;

CREATE INDEX ix_blocks_inserted_at ON blocks (inserted_at_num);
CREATE INDEX ix_transactions_inserted_at ON transactions (inserted_at_num);
CREATE INDEX ix_block_transactions_map_inserted_at ON block_transactions_map (inserted_at_num);
