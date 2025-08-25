ALTER TABLE callbacker.callbacks ADD COLUMN sent_at TIMESTAMPTZ;

CREATE INDEX ix_callbacks_sent_at ON callbacker.callbacks (sent_at);
ALTER TABLE callbacker.callbacks ADD CONSTRAINT unique_url_tx_id_status UNIQUE (url, tx_id, tx_status);
