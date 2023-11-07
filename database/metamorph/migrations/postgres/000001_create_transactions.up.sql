CREATE TABLE IF NOT EXISTS transactions (
                                            hash BYTEA PRIMARY KEY,
                                            stored_at TEXT,
                                            announced_at TEXT,
                                            mined_at TEXT,
                                            status INTEGER,
                                            block_height BIGINT,
                                            block_hash BYTEA,
                                            callback_url TEXT,
                                            callback_token TEXT,
                                            merkle_proof TEXT,
                                            reject_reason TEXT,
                                            raw_tx BYTEA
);
