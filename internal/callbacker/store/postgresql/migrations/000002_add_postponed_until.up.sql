ALTER TABLE callbacker.callbacks ADD COLUMN postponed_until TIMESTAMPTZ;

CREATE INDEX ix_callbacks_postponed_until ON callbacker.callbacks (postponed_until);
