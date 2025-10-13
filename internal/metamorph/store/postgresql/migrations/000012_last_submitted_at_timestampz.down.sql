ALTER TABLE global.Transactions ADD COLUMN inserted_at_num INTEGER NOT NULL DEFAULT 0;

UPDATE global.Transactions
SET inserted_at_num = TO_NUMBER(TO_CHAR(last_submitted_at, 'yyyymmddhh24'),'9999999999');

CREATE INDEX ix_metamorph_transactions_inserted_at_num ON global.Transactions (inserted_at_num);

DROP INDEX metamorph.ix_metamorph_transactions_last_submitted_at;
ALTER TABLE global.Transactions DROP COLUMN last_submitted_at;
