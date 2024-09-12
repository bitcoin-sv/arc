ALTER TABLE callbacker.callbacks ADD COLUMN quarantine_until TIMESTAMPTZ;

CREATE INDEX ix_callbacks_quarantine_until ON callbacker.callbacks (quarantine_until);
