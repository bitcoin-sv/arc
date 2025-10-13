ALTER TABLE global.Transactions ADD COLUMN last_submitted_at TIMESTAMPTZ;

UPDATE global.Transactions
SET last_submitted_at = TO_TIMESTAMP(inserted_at_num::text, 'YYYYMMDDHH24');

CREATE INDEX ix_metamorph_transactions_last_submitted_at ON global.Transactions (last_submitted_at);

DROP INDEX metamorph.ix_metamorph_transactions_inserted_at_num;
ALTER TABLE global.Transactions DROP COLUMN inserted_at_num;
