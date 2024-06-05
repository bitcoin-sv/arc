ALTER TABLE metamorph.transactions ADD COLUMN last_submitted_at_num INTEGER NOT NULL DEFAULT 0;

UPDATE metamorph.transactions
SET last_submitted_at_num = TO_NUMBER(TO_CHAR(last_submitted_at, 'yyyymmddhh24'),'9999999999') ;

CREATE INDEX ix_metamorph_transactions_last_submitted_at_num ON metamorph.transactions (last_submitted_at_num);

DROP INDEX IF EXISTS metamorph.ix_metamorph_transactions_last_submitted_at;
ALTER TABLE metamorph.transactions DROP COLUMN last_submitted_at;
