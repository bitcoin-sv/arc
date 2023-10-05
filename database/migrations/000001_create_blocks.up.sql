CREATE TABLE blocks (
                        inserted_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
                        id bigint NOT NULL,
                        hash bytea NOT NULL,
                        prevhash bytea NOT NULL,
                        merkleroot bytea NOT NULL,
                        height bigint NOT NULL,
                        processed_at timestamp with time zone,
                        size bigint,
                        tx_count bigint,
                        orphanedyn boolean DEFAULT false NOT NULL,
                        merkle_path text DEFAULT ''::text
);