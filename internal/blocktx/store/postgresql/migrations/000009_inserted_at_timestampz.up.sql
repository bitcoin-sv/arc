-- add inserted_at timestampz to transactions table
ALTER TABLE transactions
ADD COLUMN inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

UPDATE transactions
SET inserted_at = TO_TIMESTAMP(inserted_at_num::text, 'YYYYMMDDHH24');

DROP INDEX ix_transactions_inserted_at;
ALTER TABLE transactions DROP COLUMN inserted_at_num;

CREATE INDEX ix_transactions_inserted_at ON transactions (inserted_at);

-- add inserted_at timestampz to block_transactions_map table
ALTER TABLE block_transactions_map
ADD COLUMN inserted_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP;

UPDATE block_transactions_map
SET inserted_at = TO_TIMESTAMP(inserted_at_num::text, 'YYYYMMDDHH24');

DROP INDEX ix_block_transactions_map_inserted_at;
ALTER TABLE block_transactions_map DROP COLUMN inserted_at_num;

CREATE INDEX ix_block_transactions_map_inserted_at ON block_transactions_map (inserted_at);
