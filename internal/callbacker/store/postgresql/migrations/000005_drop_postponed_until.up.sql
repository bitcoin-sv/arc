DROP INDEX callbacker.ix_callbacks_postponed_until;

ALTER TABLE callbacker.callbacks DROP COLUMN postponed_until;
