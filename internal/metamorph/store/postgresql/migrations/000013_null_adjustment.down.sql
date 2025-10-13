ALTER TABLE global.Transactions ALTER COLUMN stored_at DROP NOT NULL;

ALTER TABLE global.Transactions ALTER COLUMN last_submitted_at DROP NOT NULL;

ALTER TABLE global.Transactions ALTER COLUMN locked_by DROP NOT NULL;

ALTER TABLE global.Transactions ALTER COLUMN full_status_updates DROP NOT NULL;
