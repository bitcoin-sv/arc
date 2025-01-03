CREATE TABLE blocktx.block_transactions (
    id BIGSERIAL PRIMARY KEY,
    block_id BIGINT,
    hash BYTEA NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    CONSTRAINT fk_block
        FOREIGN KEY(block_id)
            REFERENCES blocktx.blocks(id)
);

CREATE INDEX ix_block_transactions_hash ON blocktx.block_transactions (hash);


CREATE TABLE blocktx.registered_transactions (
--     id BIGSERIAL PRIMARY KEY,
    hash BYTEA NOT NULL,
    inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (hash)
);

-- CREATE INDEX ix_registered_transactions_hash ON blocktx.registered_transactions (hash);
