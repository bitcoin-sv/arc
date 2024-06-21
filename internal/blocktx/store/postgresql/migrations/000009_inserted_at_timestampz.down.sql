ALTER TABLE transactions ADD COLUMN inserted_at_num INTEGER DEFAULT TO_NUMBER(TO_CHAR((NOW()) AT TIME ZONE 'UTC', 'yyyymmddhh24'), '9999999999') NOT NULL;

UPDATE transactions
SET inserted_at_num = TO_NUMBER(TO_CHAR(inserted_at, 'yyyymmddhh24'),'9999999999');

DROP INDEX ix_transactions_inserted_at;
ALTER TABLE transactions DROP COLUMN inserted_at;

CREATE INDEX ix_transactions_inserted_at ON transactions (inserted_at_num);
