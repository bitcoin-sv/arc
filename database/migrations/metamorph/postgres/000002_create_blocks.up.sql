CREATE TABLE blocks (
                                  hash BYTEA PRIMARY KEY,
                                  processed_at TIMESTAMPTZ,
                                  inserted_at_num INTEGER DEFAULT TO_NUMBER(TO_CHAR((NOW()) AT TIME ZONE 'UTC', 'yyyymmddhh24'), '9999999999') NOT NULL
);

