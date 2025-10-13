-- Down Migration: Remove columns 'status_history' and 'last_modified',
-- and re-add columns 'mined_at' and 'announced_at' to 'global.Transactions' table

ALTER TABLE global.Transactions
    DROP COLUMN status_history,
    DROP COLUMN last_modified;

ALTER TABLE global.Transactions
    ADD COLUMN mined_at TIMESTAMPTZ,
    ADD COLUMN announced_at TIMESTAMPTZ;
