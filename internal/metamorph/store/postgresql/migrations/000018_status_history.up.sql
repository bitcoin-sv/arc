-- Up Migration: Add columns 'status_history' (JSONB) and 'last_modified' (TIMESTAMPTZ),
-- and remove columns 'mined_at' and 'announced_at' from 'metamorph.transactions' table

ALTER TABLE metamorph.transactions
    ADD COLUMN status_history JSONB,
    ADD COLUMN last_modified TIMESTAMPTZ;

ALTER TABLE metamorph.transactions
    DROP COLUMN mined_at,
    DROP COLUMN announced_at;
