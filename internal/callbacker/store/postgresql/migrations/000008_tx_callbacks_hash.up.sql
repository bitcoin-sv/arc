ALTER TABLE callbacker.transaction_callbacks ADD COLUMN hash BYTEA;

UPDATE callbacker.transaction_callbacks SET hash = reverse_bytes(decode(tx_id, 'hex')) WHERE hash IS NULL;

CREATE INDEX ix_transaction_callbacks_hash ON callbacker.transaction_callbacks (hash);

ALTER TABLE callbacker.transaction_callbacks ALTER COLUMN hash SET NOT NULL;
