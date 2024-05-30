ALTER TABLE metamorph.transactions RENAME COLUMN inserted_at_num TO last_submitted_at;

DROP INDEX ix_metamorph_transactions_inserted_at_num;
CREATE INDEX ix_metamorph_transactions_last_submitted_at ON metamorph.transactions (last_submitted_at);
