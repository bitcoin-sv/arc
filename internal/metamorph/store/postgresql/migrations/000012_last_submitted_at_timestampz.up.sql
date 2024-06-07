ALTER TABLE metamorph.transactions ADD COLUMN last_submitted_at TIMESTAMPTZ;

UPDATE metamorph.transactions
SET last_submitted_at = TO_TIMESTAMP(inserted_at_num::text, 'YYYYMMDDHH24');

CREATE INDEX ix_metamorph_transactions_last_submitted_at ON metamorph.transactions (last_submitted_at);

DROP INDEX metamorph.ix_metamorph_transactions_inserted_at_num;
ALTER TABLE metamorph.transactions DROP COLUMN inserted_at_num;
