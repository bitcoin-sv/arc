DROP INDEX callbacker.ix_callbacks_quarantine_until

ALTER TABLE callbacker.callbacks DROP COLUMN quarantine_until;
