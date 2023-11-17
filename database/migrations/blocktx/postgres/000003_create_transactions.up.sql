CREATE TABLE IF NOT EXISTS transactions (
    id BIGSERIAL PRIMARY KEY,
    hash BYTEA NOT NULL,
    source TEXT,
    merkle_path TEXT DEFAULT('')
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_transactions_hash ON transactions (hash);
