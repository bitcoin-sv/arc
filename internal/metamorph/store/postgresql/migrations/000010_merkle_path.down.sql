ALTER TABLE metamorph.transactions DROP COLUMN merkle_path;

CREATE TABLE metamorph.blocks (
                                  hash BYTEA PRIMARY KEY,
                                  processed_at TIMESTAMPTZ,
                                  inserted_at_num INTEGER DEFAULT TO_NUMBER(TO_CHAR((NOW()) AT TIME ZONE 'UTC', 'yyyymmddhh24'), '9999999999') NOT NULL
);

CREATE INDEX ix_metamorph_blocks_inserted_at_num ON metamorph.blocks (inserted_at_num);
