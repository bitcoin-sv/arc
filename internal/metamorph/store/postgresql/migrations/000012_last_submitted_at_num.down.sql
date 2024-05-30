ALTER TABLE metamorph.transactions RENAME COLUMN last_submitted_at TO inserted_at_num;

DROP INDEX ix_metamorph_transactions_last_submitted_at;
CREATE INDEX ix_metamorph_transactions_inserted_at_num ON metamorph.transactions (inserted_at_num);
