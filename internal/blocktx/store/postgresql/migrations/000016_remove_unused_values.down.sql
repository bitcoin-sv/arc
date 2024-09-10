ALTER TABLE blocktx.block_transactions_map ADD COLUMN pos BIGINT NOT NULL;
ALTER TABLE blocktx.transactions ADD COLUMN source TEXT;

CREATE TABLE primary_blocktx (
    host_name TEXT PRIMARY KEY,
    primary_until TIMESTAMP
);
