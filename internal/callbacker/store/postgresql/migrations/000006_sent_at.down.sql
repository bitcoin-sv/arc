DROP INDEX callbacker.ix_callbacks_sent_at;
ALTER TABLE callbacker.callbacks DROP COLUMN sent_at;
ALTER TABLE callbacker.callbacks DROP CONSTRAINT unique_tx_id_status;
