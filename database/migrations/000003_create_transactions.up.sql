CREATE TABLE transactions (
                              id BIGINT NOT NULL,
                              hash BYTEA NOT NULL,
                              source TEXT,
                              merkle_path TEXT DEFAULT ''::TEXT
);