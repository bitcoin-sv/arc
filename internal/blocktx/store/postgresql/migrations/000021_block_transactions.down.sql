
DROP TABLE blocktx.block_transactions;
DROP TABLE blocktx.registered_transactions;

DROP INDEX ix_registered_transactions_inserted_at;

CREATE TABLE blocktx.block_transactions_map (
    blockid int8 NOT NULL,
    txid int8 NOT NULL,
    inserted_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
    merkle_path text DEFAULT ''::text NULL,
    CONSTRAINT block_transactions_map_pkey PRIMARY KEY (blockid, txid)
);

CREATE INDEX ix_block_transactions_map_inserted_at ON blocktx.block_transactions_map USING btree (inserted_at);

CREATE TABLE blocktx.transactions (
    id bigserial NOT NULL,
    hash bytea NOT NULL,
    is_registered bool DEFAULT false NOT NULL,
    inserted_at timestamptz DEFAULT CURRENT_TIMESTAMP NOT NULL,
CONSTRAINT transactions_pkey PRIMARY KEY (id)
);
CREATE INDEX ix_transactions_inserted_at ON blocktx.transactions USING btree (inserted_at);
CREATE UNIQUE INDEX ux_transactions_hash ON blocktx.transactions USING btree (hash);
