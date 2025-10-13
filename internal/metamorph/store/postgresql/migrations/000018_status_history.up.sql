-- Up Migration: Add columns 'status_history' (JSONB) and 'last_modified' (TIMESTAMPTZ),
-- and remove columns 'mined_at' and 'announced_at' from 'global.Transactions' table

ALTER TABLE global.Transactions
    ADD COLUMN status_history JSONB,
    ADD COLUMN last_modified TIMESTAMPTZ;

ALTER TABLE global.Transactions
    DROP COLUMN mined_at,
    DROP COLUMN announced_at;
