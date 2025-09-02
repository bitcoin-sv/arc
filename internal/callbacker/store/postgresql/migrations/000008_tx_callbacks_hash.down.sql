DROP INDEX callbacker.ix_transaction_callbacks_hash;

ALTER TABLE callbacker.transaction_callbacks DROP COLUMN hash;
