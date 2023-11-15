CREATE TABLE transactions (
                              id INTEGER PRIMARY KEY,
                              hash BLOB NOT NULL,
                              source TEXT,
                              merkle_path TEXT DEFAULT ''
);

CREATE INDEX ix_transactions_source ON transactions (source);
CREATE INDEX ix_transactions_hash ON transactions (hash);
