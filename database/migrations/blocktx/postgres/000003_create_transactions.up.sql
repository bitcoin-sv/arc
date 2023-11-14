CREATE TABLE transactions (
                              id BIGINT NOT NULL,
                              hash BYTEA NOT NULL,
                              source TEXT,
                              merkle_path TEXT DEFAULT ''::TEXT
);

CREATE INDEX ix_transactions_source ON public.transactions USING btree (source);
CREATE INDEX ix_transactions_hash ON public.transactions USING btree (hash);
