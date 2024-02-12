CREATE TABLE transactions (
    id BIGSERIAL PRIMARY KEY,
    hash BYTEA NOT NULL,
    source TEXT,
    merkle_path TEXT DEFAULT('')
);

CREATE UNIQUE INDEX ux_transactions_hash ON transactions (hash);
