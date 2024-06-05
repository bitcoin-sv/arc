ALTER TABLE metamorph.transactions ADD COLUMN last_submitted_at TIMESTAMPTZ;

UPDATE metamorph.transactions
SET last_submitted_at = TO_TIMESTAMP(last_submitted_at_num::text, 'YYYYMMDDHH24');

CREATE INDEX ix_metamorph_transactions_last_submitted_at ON metamorph.transactions (last_submitted_at);

DROP INDEX IF EXISTS metamorph.ix_metamorph_transactions_last_submitted_at_num;
ALTER TABLE metamorph.transactions DROP COLUMN last_submitted_at_num;