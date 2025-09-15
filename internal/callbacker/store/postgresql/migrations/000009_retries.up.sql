ALTER TABLE callbacker.transaction_callbacks ADD COLUMN retries INTEGER NOT NULL DEFAULT 0;
ALTER TABLE callbacker.transaction_callbacks ADD COLUMN disable BOOLEAN NOT NULL DEFAULT FALSE;
