CREATE TABLE blocks (
                        inserted_at TIMESTAMP WITH TIME ZONE  DEFAULT CURRENT_TIMESTAMP NOT NULL,
                        id BIGSERIAL PRIMARY KEY,
                        hash BYTEA NOT NULL,
                        prevhash BYTEA NOT NULL,
                        merkleroot BYTEA NOT NULL,
                        height BIGINT NOT NULL,
                        processed_at TIMESTAMP WITH TIME ZONE,
                        size BIGINT,
                        tx_count BIGINT,
                        orphanedyn BOOLEAN DEFAULT FALSE NOT NULL,
                        merkle_path TEXT DEFAULT ''::TEXT
);

CREATE INDEX ix_transactions ON public.blocks USING btree (hash);